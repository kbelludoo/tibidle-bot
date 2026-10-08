package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"tibidle-bot/pkg/client"
	"tibidle-bot/pkg/protocol"
)

type Bot struct {
	config AccountConfig
	status *SafeStatus

	sessionClient *client.SessionClient
	socket        *client.SocketClient

	mu               sync.Mutex
	ticketChan       chan string
	currentHuntID    int
	isHunting        bool
	lastActionTime   time.Time
	lastDailyCheck   time.Time
	lastBlessingBuy  time.Time
	lastSellTime     time.Time
}

func NewBot(cfg AccountConfig) *Bot {
	huntID := cfg.HuntID
	if huntID == 0 && cfg.HuntName != "" {
		if def := protocol.FindHuntByName(cfg.HuntName); def != nil {
			huntID = def.ID
		}
	}
	if huntID == 0 {
		huntID = 1 // default hunt if none specified
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
		ticketChan:    make(chan string, 10),
		currentHuntID: huntID,
	}
}

func (b *Bot) SupplyTicket(ticket string) {
	b.mu.Lock()
	b.config.Ticket = ticket
	b.mu.Unlock()
	b.status.AddLog("Ticket recebido externamente! Conectando...")
	select {
	case b.ticketChan <- ticket:
	default:
	}
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

	// 2. Validar conta
	acc, err := b.sessionClient.ValidateSession(cookies)
	if err != nil {
		return fmt.Errorf("sessão inválida: %w", err)
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
	wsURL := "wss://play.tibidle.com/"

	if ticket == "" {
		var err error
		ticket, wsURL, err = b.sessionClient.GetGameTicket(cookies, b.config.SessionProof)
		if err != nil {
			log.Printf("[%s] %v. Aguardando ticket via POST /api/ticket ou accounts.json...", b.config.ID, err)
			b.status.Update(func(st *AccountStatus) {
				st.Status = "Aguardando Ticket (envie via push ou Web)"
				st.UpdatedAt = time.Now().UnixMilli()
			})
			select {
			case <-ctx.Done():
				return ctx.Err()
			case t := <-b.ticketChan:
				ticket = t
			case <-time.After(30 * time.Second):
				return fmt.Errorf("aguardando ticket: %w", err)
			}
		}
	}

	// Limpar da config para forçar ticket novo em reconexão futura
	b.mu.Lock()
	b.config.Ticket = ""
	b.mu.Unlock()

	if wsURL == "" {
		wsURL = "wss://play.tibidle.com/"
	}

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

	// Pedir snapshot inicial
	_ = sock.Send("party_get_snapshot", map[string]any{})
	_ = sock.Send("eventos_get", map[string]any{"requestId": fmt.Sprintf("req-%d", time.Now().UnixNano())})

	for {
		select {
		case <-ctx.Done():
			sock.Close()
			return nil
		case err := <-disconnectChan:
			return fmt.Errorf("websocket desconectou: %v", err)
		case <-actionTicker.C:
			b.performPeriodicActions()
			b.persistStatus()
		}
	}
}

func (b *Bot) handleMessage(msg protocol.InboundMessage) {
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
			}
			b.mu.Unlock()

			b.status.Update(func(st *AccountStatus) {
				st.Status = ps.Status
				st.CurrentHunt = huntTitle
				st.UpdatedAt = time.Now().UnixMilli()
			})
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
		b.mu.Lock()
		b.isHunting = false
		b.mu.Unlock()
		b.status.AddLog("Caçada finalizada! Preparando próxima caçada...")
		b.status.Update(func(st *AccountStatus) {
			st.Status = "idle"
			st.UpdatedAt = time.Now().UnixMilli()
		})

	case "eventos_state", "eventos_result":
		// Resposta de eventos diários
		b.claimDailyEvents()
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
	huntID := b.currentHuntID
	lure := b.config.Lure
	b.mu.Unlock()

	if sock == nil {
		return
	}

	// 1. Auto-Hunt se estiver idle
	if !isHunting && time.Since(b.lastActionTime) > 6*time.Second {
		b.lastActionTime = time.Now()
		b.status.AddLog(fmt.Sprintf("Iniciando caçada (Hunt ID: %d, Lure: %d)...", huntID, lure))

		huntPayload := protocol.StartHuntPayload{
			HuntID:   huntID,
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
