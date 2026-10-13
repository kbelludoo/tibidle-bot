package bot

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"tibidle-bot/pkg/client"
	"tibidle-bot/pkg/protocol"
)

type TicketSubmission struct {
	Ticket string
	WSURL  string
	At     time.Time
}

type Bot struct {
	config AccountConfig
	status *SafeStatus

	sessionClient *client.SessionClient
	socket        *client.SocketClient

	mu               sync.Mutex
	ticketChan       chan TicketSubmission
	sessionDirty     bool
	configPath       string
	currentHuntID    int
	isHunting        bool
	lastActionTime   time.Time
	lastDailyCheck   time.Time
	lastBlessingBuy  time.Time
	lastSellTime     time.Time
	benchmarks       map[int]*HuntBenchmark
	bestiaryKills    map[string]int
	consecutiveFails int
}

// HuntTier defines the recommended progression hunt for level brackets
type HuntTier struct {
	MinLevel int
	HuntID   int
	HuntName string
	Lure     int
}

// Default bestiary target per creature tier (e.g. 500 kills)
const BestiaryTargetKills = 500

// BestiaryEligibleHunts lists hunts suitable for bestiary completion by level
var BestiaryEligibleHunts = []HuntTier{
	{MinLevel: 1, HuntID: 26, HuntName: "Daramian Minotaur Pyramid", Lure: 2}, // Minotaur
	{MinLevel: 1, HuntID: 170, HuntName: "Wolves Den", Lure: 2},                 // Wolf
	{MinLevel: 1, HuntID: 176, HuntName: "Wasps Dungeons", Lure: 2},             // Wasp
	{MinLevel: 1, HuntID: 125, HuntName: "Goblins Femor Hills", Lure: 2},        // Goblin
	{MinLevel: 1, HuntID: 126, HuntName: "Rotworms Darashia", Lure: 2},          // Rotworm
	{MinLevel: 10, HuntID: 131, HuntName: "Dwarven Mines", Lure: 2},            // Dwarf
	{MinLevel: 15, HuntID: 128, HuntName: "Larva Ankrahmuns", Lure: 2},          // Larva
	{MinLevel: 20, HuntID: 139, HuntName: "Crocodiles Port Hope", Lure: 2},      // Crocodile
	{MinLevel: 20, HuntID: 180, HuntName: "Amazon Camp", Lure: 1},               // Amazon, Valkyrie
	{MinLevel: 25, HuntID: 32, HuntName: "Elfs Shadowthorn", Lure: 3},           // Elf Scout
	{MinLevel: 30, HuntID: 134, HuntName: "Dworcs Port Hope", Lure: 3},          // Dworcs
	{MinLevel: 35, HuntID: 141, HuntName: "Cyclops Thais", Lure: 1},             // Cyclops
	{MinLevel: 45, HuntID: 140, HuntName: "Scarabs Cave", Lure: 2},              // Scarabs
	{MinLevel: 50, HuntID: 43, HuntName: "Cults Goroma", Lure: 2},               // Cultists
	{MinLevel: 60, HuntID: 46, HuntName: "Dragon Lair", Lure: 3},                // Dragons
}

// RecommendedProgressionTiers contains the optimal party progression order for pure XP
var RecommendedProgressionTiers = []HuntTier{
	{MinLevel: 1, HuntID: 26, HuntName: "Daramian Minotaur Pyramid", Lure: 2}, // Lure 2 (safe, high spawn)
	{MinLevel: 30, HuntID: 32, HuntName: "Elfs Shadowthorn", Lure: 3},         // Lure 3 (Elf Scout, bow loot, fast exp)
	{MinLevel: 45, HuntID: 140, HuntName: "Scarabs Cave", Lure: 2},            // Lure 2 (Scarabs, safe at 45+)
	{MinLevel: 60, HuntID: 46, HuntName: "Dragon Lair", Lure: 3},              // Lure 3 (Dragons, high exp & rares)
	{MinLevel: 75, HuntID: 106, HuntName: "Hero Fortress Entrance", Lure: 3},  // Lure 3 (Bonebeast, Necro, Squire)
	{MinLevel: 90, HuntID: 49, HuntName: "Pits of Inferno Entrance", Lure: 3}, // Lure 3 (DLs, massive exp)
	{MinLevel: 110, HuntID: 192, HuntName: "Draken Walls", Lure: 3},           // Lure 3 (Drakens)
	{MinLevel: 130, HuntID: 148, HuntName: "Warzone - Lava Lurkers", Lure: 4}, // Lure 4 (Top tier exp)
}

func GetProgressionHuntForLevel(level int) HuntTier {
	chosen := RecommendedProgressionTiers[0]
	for _, tier := range RecommendedProgressionTiers {
		if level >= tier.MinLevel {
			chosen = tier
		}
	}
	return chosen
}

func ResolveWSURL(ticket string, providedWSURL string) string {
	if providedWSURL != "" {
		return providedWSURL
	}
	parts := strings.Split(ticket, ".")
	if len(parts) >= 2 {
		seg := parts[1]
		if l := len(seg) % 4; l > 0 {
			seg += strings.Repeat("=", 4-l)
		}
		payloadBytes, err := base64.URLEncoding.DecodeString(seg)
		if err == nil {
			var claims struct {
				Chn string `json:"chn"`
			}
			if err := json.Unmarshal(payloadBytes, &claims); err == nil && claims.Chn != "" {
				chnNum := strings.TrimPrefix(claims.Chn, "c")
				if chnNum != "" && chnNum != "1" {
					return fmt.Sprintf("wss://play%s.tibidle.com/", chnNum)
				}
			}
		}
	}
	return "wss://play.tibidle.com/"
}

func NewBot(cfg AccountConfig) *Bot {
	huntID := cfg.HuntID
	if huntID == 0 && cfg.HuntName != "" {
		if def := protocol.FindHuntByName(cfg.HuntName); def != nil {
			huntID = def.ID
		}
	}
	if huntID == 0 {
		huntID = 26 // default Daramian Minotaurs
	}

	lure := cfg.Lure
	if lure <= 0 {
		lure = 1
	}
	cfg.Lure = lure
	cfg.HuntID = huntID

	return &Bot{
		config:        cfg,
		status:        NewSafeStatus(cfg.ID),
		sessionClient: client.NewSessionClient(),
		ticketChan:    make(chan TicketSubmission, 10),
		currentHuntID: huntID,
		benchmarks:    make(map[int]*HuntBenchmark),
		bestiaryKills: make(map[string]int),
	}
}

func (b *Bot) SupplyTicket(ticket, wsURL string) {
	if wsURL == "" {
		wsURL = ResolveWSURL(ticket, "")
	}
	b.mu.Lock()
	b.config.Ticket = ticket
	b.mu.Unlock()
	b.status.AddLog(fmt.Sprintf("Ticket recebido externamente (WS: %s)! Conectando...", wsURL))
	select {
	case b.ticketChan <- TicketSubmission{Ticket: ticket, WSURL: wsURL, At: time.Now()}:
	default:
	}
}

// SetConfigPath define o arquivo accounts.json desta conta para persistir
// cookies renovados (login automático) de volta em disco.
func (b *Bot) SetConfigPath(path string) {
	b.configPath = path
}

func (b *Bot) saveSessionCookie() {
	if b.configPath == "" {
		return
	}
	content, err := os.ReadFile(b.configPath)
	if err != nil {
		return
	}
	var arr []AccountConfig
	if err := json.Unmarshal(content, &arr); err == nil {
		found := false
		for i := range arr {
			if arr[i].ID == b.config.ID || (len(arr) == 1 && arr[i].ID == "") {
				arr[i].SessionCookie = b.config.SessionCookie
				arr[i].SessionProof = b.config.SessionProof
				found = true
			}
		}
		if !found {
			return
		}
		if out, err := json.MarshalIndent(arr, "", "  "); err == nil {
			_ = os.WriteFile(b.configPath, out, 0644)
		}
		return
	}
	var single AccountConfig
	if err := json.Unmarshal(content, &single); err == nil && single.ID == b.config.ID {
		single.SessionCookie = b.config.SessionCookie
		single.SessionProof = b.config.SessionProof
		if out, err := json.MarshalIndent(single, "", "  "); err == nil {
			_ = os.WriteFile(b.configPath, out, 0644)
		}
	}
}

// forceRelogin marca a sessão como suja para o próximo ciclo de conexão.
func (b *Bot) forceRelogin() {
	b.mu.Lock()
	b.sessionDirty = true
	b.mu.Unlock()
}

func (b *Bot) GetStatus() AccountStatus {
	return b.status.Get()
}

func (b *Bot) Run(ctx context.Context) {
	b.status.AddLog(fmt.Sprintf("Bot iniciado para ID: %s", b.config.ID))

	for {
		select {
		case <-ctx.Done():
			b.status.Update(func(st *AccountStatus) { st.Status = "Desconectado" })
			if b.socket != nil {
				b.socket.Close()
			}
			return
		default:
			if err := b.connectAndPlay(ctx); err != nil {
				log.Printf("[%s] Erro na sessão: %v. Reconectando em 10s...", b.config.ID, err)
				b.status.Update(func(st *AccountStatus) {
					st.Status = fmt.Sprintf("Reconectando (%v)", err)
					st.UpdatedAt = time.Now().UnixMilli()
				})
				b.persistStatus()

				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Second):
				}
			}
		}
	}
}

func (b *Bot) connectAndPlay(ctx context.Context) error {
	cookies := b.config.SessionCookie

	// 1. Obter ou validar cookies
	if cookies == "" && b.config.Email != "" && b.config.Password != "" {
		b.status.AddLog("Fazendo login via email e senha...")
		var err error
		cookies, _, err = b.sessionClient.LoginWithEmailPassword(b.config.Email, b.config.Password)
		if err != nil {
			return fmt.Errorf("falha no login: %w", err)
		}
		b.config.SessionCookie = cookies
	}

	if cookies == "" {
		return fmt.Errorf("nenhum cookie de sessão ou credenciais configuradas")
	}

	// 2. Validar conta; se sessão morreu (401) ou foi invalidada pelo servidor,
	// renova automaticamente via login de senha antes de desistir.
	b.mu.Lock()
	dirty := b.sessionDirty
	b.sessionDirty = false
	b.mu.Unlock()

	if dirty {
		b.status.AddLog("Sessão marcada como inválida pelo servidor. Renovando via login...")
		if newCookies, _, err := b.sessionClient.LoginWithEmailPassword(b.config.Email, b.config.Password); err == nil {
			cookies = newCookies
			b.config.SessionCookie = newCookies
			b.saveSessionCookie()
			b.status.AddLog("Sessão renovada via login automático.")
		} else {
			log.Printf("[%s] Falha ao renovar sessão: %v", b.config.ID, err)
		}
	}

	acc, err := b.sessionClient.ValidateSession(cookies)
	if err != nil {
		if b.config.Email == "" || b.config.Password == "" {
			return fmt.Errorf("sessão inválida: %w", err)
		}
		b.status.AddLog("Sessão expirou. Fazendo login automático...")
		newCookies, _, lerr := b.sessionClient.LoginWithEmailPassword(b.config.Email, b.config.Password)
		if lerr != nil {
			return fmt.Errorf("sessão inválida e login falhou: %v (original: %w)", lerr, err)
		}
		cookies = newCookies
		b.config.SessionCookie = newCookies
		b.saveSessionCookie()
		b.status.AddLog("Login automático OK. Nova sessão salva.")
		acc, err = b.sessionClient.ValidateSession(cookies)
		if err != nil {
			return fmt.Errorf("sessão inválida mesmo após login: %w", err)
		}
	}

	b.status.Update(func(st *AccountStatus) {
		st.Name = acc.Name
		st.Status = "Autenticado. Obtendo ticket de jogo..."
		st.UpdatedAt = time.Now().UnixMilli()
	})
	b.status.AddLog(fmt.Sprintf("Conectado como %s (%s)", acc.Name, acc.Email))

	// 3. Obter Ticket do Jogo
	b.mu.Lock()
	ticket := b.config.Ticket
	b.mu.Unlock()
	wsURL := ""

	if ticket == "" {
		var terr error
		ticket, wsURL, terr = b.sessionClient.GetGameTicket(cookies, b.config.SessionProof)
		if terr != nil {
			log.Printf("[%s] %v. Aguardando ticket via POST /api/ticket (relay automático ou Web)...", b.config.ID, terr)
			b.status.Update(func(st *AccountStatus) {
				st.Status = "Aguardando Ticket (envie via push ou Web)"
				st.UpdatedAt = time.Now().UnixMilli()
			})
			// Relay de PC leva até ~45s (poll + geração no navegador + push).
			// Ticket TTL é 60s, então descarta qualquer ticket com mais de 45s.
			deadline := time.After(90 * time.Second)
		waitTicket:
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case sub := <-b.ticketChan:
					if sub.At.IsZero() || time.Since(sub.At) <= 45*time.Second {
						ticket = sub.Ticket
						wsURL = sub.WSURL
						break waitTicket
					}
					b.status.AddLog("Ticket expirado descartado (>45s). Aguardando outro...")
				case <-deadline:
					return fmt.Errorf("aguardando ticket: %w", terr)
				}
			}
		}
	}

	// Limpar da config para forçar ticket novo em reconexão futura
	b.mu.Lock()
	b.config.Ticket = ""
	b.mu.Unlock()

	wsURL = ResolveWSURL(ticket, wsURL)

	b.status.Update(func(st *AccountStatus) {
		st.Status = "Conectando ao WebSocket..."
		st.UpdatedAt = time.Now().UnixMilli()
	})

	// 4. Conectar WebSocket
	disconnectChan := make(chan error, 1)
	sock := client.NewSocketClient(
		wsURL,
		ticket,
		func(msg protocol.InboundMessage) { b.handleMessage(msg) },
		func(err error) { disconnectChan <- err },
	)

	b.mu.Lock()
	b.socket = sock
	b.mu.Unlock()

	if err := sock.Connect(); err != nil {
		return fmt.Errorf("falha ao conectar WebSocket: %w", err)
	}

	b.status.Update(func(st *AccountStatus) {
		st.Status = "Conectado"
		st.UpdatedAt = time.Now().UnixMilli()
	})
	b.status.AddLog("Conexão WebSocket estabelecida com sucesso!")

	// 5. Iniciar loop de ações periódicas
	actionTicker := time.NewTicker(3 * time.Second)
	defer actionTicker.Stop()

	// Pedir status de eventos inicial
	_ = sock.Send("eventos_get", map[string]any{"requestId": fmt.Sprintf("req-%d", time.Now().UnixNano())})

	for {
		select {
		case <-ctx.Done():
			sock.Close()
			return nil
		case err := <-disconnectChan:
			if err != nil {
				var ce *websocket.CloseError
				if errors.As(err, &ce) {
					log.Printf("[%s] WebSocket fechado pelo servidor: code=%d reason=%q", b.config.ID, ce.Code, ce.Text)
					b.status.AddLog(fmt.Sprintf("Servidor fechou conexão (code %d)", ce.Code))
					// 4401 unauthorized / 4403 session_replaced: força novo login no próximo ciclo
					if ce.Code == 4401 || ce.Code == 4403 {
						b.forceRelogin()
					}
				} else {
					log.Printf("[%s] WebSocket desconectou: %v", b.config.ID, err)
				}
				return fmt.Errorf("websocket desconectou: %v", err)
			}
			return fmt.Errorf("websocket desconectou: conexão fechada sem código")
		case <-actionTicker.C:
			b.performPeriodicActions()
			b.persistStatus()
		}
	}
}

func (b *Bot) handleMessage(msg protocol.InboundMessage) {
	if msg.Type != "frame" && msg.Type != "keepalive" {
		log.Printf("[%s] INBOUND: type=%s data=%s", b.config.ID, msg.Type, string(msg.Data))
	}
	switch msg.Type {
	case "frame":
		var fp protocol.FramePayload
		if err := json.Unmarshal(msg.Data, &fp); err == nil {
			b.status.Update(func(st *AccountStatus) {
				ch := fp.State.Character
				if ch.Name != "" {
					st.Name = ch.Name
					st.Vocation = ch.Vocation
					st.Level = ch.Level
					st.HP = ch.HP
					st.MaxHP = ch.MaxHP
					st.Mana = ch.Mana
					st.MaxMana = ch.MaxMana
					st.Gold = ch.Gold
					st.XP = ch.XP
				}
				an := fp.Analyzer
				st.Kills = an.KillsTotal
				st.ExpPerHour = float64(an.XpPerHour)
				st.DurationSec = an.ElapsedMs / 1000
				st.UpdatedAt = time.Now().UnixMilli()
			})

			// Auto-sell se capacidade estiver cheia ou tiver itens
			if b.config.AutoSell && len(fp.State.Inventory) > 0 && time.Since(b.lastSellTime) > 30*time.Second {
				b.autoSellInventory(fp.State.Inventory)
			}
		}

	case "party_snapshot":
		var ps protocol.PartySnapshotPayload
		if err := json.Unmarshal(msg.Data, &ps); err == nil {
			b.mu.Lock()
			b.isHunting = (ps.Status == "hunting")
			huntTitle := ps.Status
			if name, ok := ps.Title.Params["name"].(string); ok && name != "" {
				huntTitle = name
				if def := protocol.FindHuntByName(name); def != nil {
					b.currentHuntID = def.ID
				}
			}

			// Descobrir nível mais alto ou médio da party
			maxLevel := 0
			var leaderVocation string
			for _, m := range ps.Members {
				if m.Level > maxLevel {
					maxLevel = m.Level
				}
				if m.IsLeader {
					leaderVocation = m.Vocation
				}
			}
			autoProg := b.config.AutoProgress
			b.mu.Unlock()

			b.status.Update(func(st *AccountStatus) {
				st.Status = ps.Status
				st.CurrentHunt = huntTitle
				if maxLevel > 0 {
					st.Level = maxLevel
				}
				if leaderVocation != "" {
					st.Vocation = leaderVocation
				}
				if ps.ElapsedSec > 0 {
					st.DurationSec = ps.ElapsedSec
				}
				st.UpdatedAt = time.Now().UnixMilli()
			})

			// Se AutoProgress estiver ativo e temos nível da party, verificar se devemos progredir
			if autoProg && maxLevel > 0 {
				rec := GetProgressionHuntForLevel(maxLevel)
				b.mu.Lock()
				// Se ainda não falhamos repetidamente nessa hunt recomendada
				if b.consecutiveFails < 2 && b.config.HuntID != rec.HuntID {
					oldHunt := b.config.HuntName
					b.config.HuntID = rec.HuntID
					b.config.HuntName = rec.HuntName
					b.config.Lure = rec.Lure
					b.status.AddLog(fmt.Sprintf("⭐ Auto-Progresso (Lv. %d): alternando de '%s' para '%s' (Lure: %d)", maxLevel, oldHunt, rec.HuntName, rec.Lure))
				}
				b.status.Update(func(st *AccountStatus) {
					st.TargetHunt = rec.HuntName
				})
				b.mu.Unlock()
			}
		}

	case "resume":
		var rp protocol.ResumePayload
		if err := json.Unmarshal(msg.Data, &rp); err == nil {
			b.mu.Lock()
			b.isHunting = true
			if rp.HuntID > 0 {
				b.currentHuntID = rp.HuntID
			}
			b.mu.Unlock()

			b.status.Update(func(st *AccountStatus) {
				st.Status = "hunting"
				if def := protocol.FindHuntByID(rp.HuntID); def != nil {
					st.CurrentHunt = def.Title
				}
				st.UpdatedAt = time.Now().UnixMilli()
			})
		}

	case "ended":
		var endPayload struct {
			Summary struct {
				HuntID int `json:"huntId"`
				Title  struct {
					Key    string         `json:"key"`
					Params map[string]any `json:"params"`
				} `json:"title"`
				Reason     string         `json:"reason"` // "death", "manual", "duration"
				ElapsedSec int            `json:"elapsedSec"`
				XpPerHour  float64        `json:"xpPerHour"`
				Kills      map[string]int `json:"kills"`
				KillsTotal int            `json:"killsTotal"`
				LootGold   int            `json:"lootGold"`
				Deaths     int            `json:"deaths"`
				Level      struct {
					Start int `json:"start"`
					End   int `json:"end"`
				} `json:"level"`
			} `json:"summary"`
		}
		_ = json.Unmarshal(msg.Data, &endPayload)

		b.mu.Lock()
		b.isHunting = false
		sum := endPayload.Summary
		huntID := sum.HuntID
		if huntID == 0 {
			huntID = b.currentHuntID
		}

		huntName := "Hunt"
		if name, ok := sum.Title.Params["name"].(string); ok && name != "" {
			huntName = name
		} else if def := protocol.FindHuntByID(huntID); def != nil {
			huntName = def.Title
		}

		// Gravar / Atualizar métricas de benchmark
		if huntID > 0 {
			bm, exists := b.benchmarks[huntID]
			if !exists {
				bm = &HuntBenchmark{
					HuntID:   huntID,
					HuntName: huntName,
				}
				b.benchmarks[huntID] = bm
			}
			bm.Runs++
			bm.LastDuration = sum.ElapsedSec
			if sum.Deaths > 0 {
				bm.Deaths += sum.Deaths
			}
			if sum.XpPerHour > bm.BestXpPerHour {
				bm.BestXpPerHour = sum.XpPerHour
			}
		}

		// Acumular abates de monstros para o Bestiário
		for monster, count := range sum.Kills {
			if count > 0 {
				b.bestiaryKills[monster] += count
			}
		}

		// Construir lista de Bestiário
		var bestiaryList []MonsterKillStat
		for monster, count := range b.bestiaryKills {
			bestiaryList = append(bestiaryList, MonsterKillStat{
				Name:      monster,
				Kills:     count,
				Target:    BestiaryTargetKills,
				Completed: count >= BestiaryTargetKills,
			})
		}

		// Lista de benchmarks para o AccountStatus
		var bmList []HuntBenchmark
		for _, v := range b.benchmarks {
			bmList = append(bmList, *v)
		}

		// Se BestiaryMode estiver ativo, verificar se monstros desta hunt já foram concluídos
		if b.config.BestiaryMode {
			def := protocol.FindHuntByID(huntID)
			if def != nil && len(def.Monsters) > 0 {
				allDone := true
				for _, m := range def.Monsters {
					if b.bestiaryKills[m] < BestiaryTargetKills {
						allDone = false
						break
					}
				}
				if allDone {
					b.status.AddLog(fmt.Sprintf("🏆 Bestiário de '%s' CONCLUÍDO! Procurando próxima hunt pendente...", huntName))
					// Encontra próxima hunt de bestiário elegível que ainda tenha monstro pendente
					for _, h := range BestiaryEligibleHunts {
						hDef := protocol.FindHuntByID(h.HuntID)
						if hDef == nil {
							continue
						}
						hasPending := false
						for _, m := range hDef.Monsters {
							if b.bestiaryKills[m] < BestiaryTargetKills {
								hasPending = true
								break
							}
						}
						if hasPending {
							b.config.HuntID = h.HuntID
							b.config.HuntName = h.HuntName
							b.config.Lure = h.Lure
							b.status.AddLog(fmt.Sprintf("🎯 Bestiário rotacionado para '%s' (Lure: %d) para farmar novos monstros!", h.HuntName, h.Lure))
							break
						}
					}
				}
			}
		}

		// Tratar mortes ou falhas
		if sum.Reason == "death" {
			b.consecutiveFails++
			log.Printf("[%s] Morte na hunt %s (ID %d). Falhas consecutivas: %d", b.config.ID, huntName, huntID, b.consecutiveFails)
			b.status.AddLog(fmt.Sprintf("⚠️ Morte na hunt '%s' após %ds! Falhas consecutivas: %d", huntName, sum.ElapsedSec, b.consecutiveFails))

			// Se morrer 2x ou mais na hunt atual, regride temporariamente para uma hunt mais segura
			if b.consecutiveFails >= 2 {
				fallback := RecommendedProgressionTiers[0] // Daramian Minotaurs
				if huntID == 140 {                         // Se morreu nos Scarabs, regride para Elfs
					fallback = RecommendedProgressionTiers[1]
				} else if huntID == 46 { // Se morreu nos Dragons, regride para Scarabs
					fallback = RecommendedProgressionTiers[2]
				}
				b.config.HuntID = fallback.HuntID
				b.config.HuntName = fallback.HuntName
				b.config.Lure = fallback.Lure
				b.currentHuntID = fallback.HuntID
				b.status.AddLog(fmt.Sprintf("🛡️ Segurança: Recuando temporariamente para '%s' (Lure: %d) para evitar novas mortes.", fallback.HuntName, fallback.Lure))
			}
		} else {
			// Caçada bem sucedida ou parada manual
			b.consecutiveFails = 0
			if sum.XpPerHour > 0 {
				b.status.AddLog(fmt.Sprintf("📊 Hunt '%s' concluída: %.0f XP/h (Duração: %ds, Kills: %d)", huntName, sum.XpPerHour, sum.ElapsedSec, sum.KillsTotal))
			} else {
				b.status.AddLog(fmt.Sprintf("Caçada '%s' finalizada! Preparando próxima...", huntName))
			}
		}
		b.mu.Unlock()

		b.status.Update(func(st *AccountStatus) {
			st.Status = "idle"
			st.Benchmarks = bmList
			st.Bestiary = bestiaryList
			if sum.XpPerHour > 0 {
				st.ExpPerHour = sum.XpPerHour
			}
			st.UpdatedAt = time.Now().UnixMilli()
		})

	case "error":
		var errData struct {
			Code   string `json:"code"`
			Key    string `json:"key"`
			Params struct {
				Min   int `json:"min"`
				Nivel int `json:"nivel"`
			} `json:"params"`
		}
		if err := json.Unmarshal(msg.Data, &errData); err == nil {
			if errData.Code == "level_too_low" {
				log.Printf("[%s] Nível insuficiente (%d < %d). Alternando hunt fallback...", b.config.ID, errData.Params.Nivel, errData.Params.Min)
				b.mu.Lock()
				fallback := RecommendedProgressionTiers[0]
				if errData.Params.Nivel >= 30 {
					fallback = RecommendedProgressionTiers[1] // Elfs
				}
				b.config.HuntID = fallback.HuntID
				b.config.HuntName = fallback.HuntName
				b.config.Lure = fallback.Lure
				b.currentHuntID = fallback.HuntID
				b.isHunting = false
				b.lastActionTime = time.Time{}
				b.consecutiveFails++
				b.mu.Unlock()
				b.status.AddLog(fmt.Sprintf("Nível baixo para hunt anterior. Alternado automaticamente para %s (ID %d)", b.config.HuntName, b.config.HuntID))
			}
		}

	case "eventos_state":
		// Apenas estado informativo de eventos
	}
}

func (b *Bot) autoSellInventory(inv []protocol.InventoryItem) {
	b.mu.Lock()
	sock := b.socket
	b.mu.Unlock()
	if sock == nil {
		return
	}

	var itemsToSell []protocol.SellItem
	for _, it := range inv {
		if it.Count > 0 {
			itemsToSell = append(itemsToSell, protocol.SellItem{
				Name:  it.Name,
				Count: it.Count,
				IID:   it.IID,
			})
		}
	}

	if len(itemsToSell) > 0 {
		b.lastSellTime = time.Now()
		b.status.AddLog(fmt.Sprintf("Auto-Sell: vendendo %d itens de loot...", len(itemsToSell)))
		_ = sock.Send("sell_loot", protocol.SellLootPayload{
			Items:     itemsToSell,
			RequestID: fmt.Sprintf("sell-%d", time.Now().UnixNano()),
		})
	}
}

func (b *Bot) performPeriodicActions() {
	b.mu.Lock()
	sock := b.socket
	isHunting := b.isHunting
	currentHuntID := b.currentHuntID
	cfgHuntID := b.config.HuntID
	lure := b.config.Lure
	b.mu.Unlock()

	if sock == nil {
		return
	}

	targetHuntID := cfgHuntID
	if targetHuntID == 0 {
		targetHuntID = currentHuntID
	}
	if targetHuntID == 0 {
		targetHuntID = 26
	}

	// Se está caçando, atualiza target para a hunt em andamento até ela finalizar naturalmente
	if isHunting {
		return
	}

	// 1. Auto-Hunt se estiver idle
	if !isHunting && time.Since(b.lastActionTime) > 5*time.Second {
		b.lastActionTime = time.Now()
		b.status.AddLog(fmt.Sprintf("Iniciando caçada (Hunt ID: %d, Lure: %d)...", targetHuntID, lure))

		huntPayload := protocol.StartHuntPayload{
			HuntID:   targetHuntID,
			Lure:     lure,
			AutoBoss: true,
		}
		_ = sock.Send("start_hunt", huntPayload)
	}

	// 2. Auto-Blessings a cada 15 minutos
	if b.config.AutoBlessings && time.Since(b.lastBlessingBuy) > 15*time.Minute {
		b.lastBlessingBuy = time.Now()
		_ = sock.Send("buy_all_blessings", protocol.BuyBlessingsPayload{
			RequestID: fmt.Sprintf("bless-%d", time.Now().UnixNano()),
		})
	}

	// 3. Auto-Daily Quests a cada 1 hora
	if b.config.AutoDaily && time.Since(b.lastDailyCheck) > 1*time.Hour {
		b.lastDailyCheck = time.Now()
		_ = sock.Send("eventos_get", map[string]any{"requestId": fmt.Sprintf("ev-%d", time.Now().UnixNano())})
	}
}

func (b *Bot) claimDailyEvents() {
	b.mu.Lock()
	sock := b.socket
	b.mu.Unlock()
	if sock == nil {
		return
	}

	// Claim slots 0, 1, 2
	for slot := 0; slot < 3; slot++ {
		_ = sock.Send("eventos_daily_claim", protocol.EventosClaimPayload{
			Slot:      slot,
			RequestID: fmt.Sprintf("claim-%d-%d", slot, time.Now().UnixNano()),
		})
	}
}

func (b *Bot) persistStatus() {
	st := b.GetStatus()
	_ = os.MkdirAll("data", 0755)
	filePath := filepath.Join("data", fmt.Sprintf("status_%s.json", b.config.ID))
	bData, err := json.MarshalIndent(st, "", "  ")
	if err == nil {
		_ = os.WriteFile(filePath, bData, 0644)
	}
}
