package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"tibidle-bot/pkg/bot"
	"tibidle-bot/pkg/server"
)

type BotManager struct {
	bots []*bot.Bot
}

func (m *BotManager) GetAllStatuses() []bot.AccountStatus {
	var list []bot.AccountStatus
	for _, b := range m.bots {
		list = append(list, b.GetStatus())
	}
	return list
}

func (m *BotManager) SupplyTicket(accountID, ticket string) error {
	if len(m.bots) == 0 {
		return fmt.Errorf("nenhum bot ativo no momento")
	}
	for _, b := range m.bots {
		st := b.GetStatus()
		if accountID == "" || st.ID == accountID || st.Name == accountID {
			b.SupplyTicket(ticket)
			return nil
		}
	}
	// Se não achou por ID/Nome específico, entrega ao primeiro bot
	m.bots[0].SupplyTicket(ticket)
	return nil
}

func formatGold(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d gp", n)
	}
	if n < 1_000_000 {
		return fmt.Sprintf("%.1fk gp", float64(n)/1000.0)
	}
	return fmt.Sprintf("%.2fkk gp", float64(n)/1_000_000.0)
}

func main() {
	log.Println("================================================================================")
	log.Println("     ⚔  TIBIDLE MULTI-BOT NATIVO EM GO (HIGH-PERFORMANCE / ULTRA-LEVE) ⚔        ")
	log.Println("================================================================================")

	var accountConfigs []bot.AccountConfig

	// 1. Carregar accounts.json ou session.json
	configFile := "accounts.json"
	if _, err := os.Stat("accounts.json"); os.IsNotExist(err) {
		if _, errS := os.Stat("session.json"); errS == nil {
			configFile = "session.json"
		}
	}

	content, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Erro: %s não encontrado! Copie accounts.example.json para %s e preencha.", configFile, configFile)
	}

	// Tentar array de contas ou conta única
	if err := json.Unmarshal(content, &accountConfigs); err != nil {
		var single bot.AccountConfig
		if errSingle := json.Unmarshal(content, &single); errSingle == nil && (single.ID != "" || single.Email != "" || single.SessionCookie != "") {
			if single.ID == "" {
				single.ID = "acc-1"
			}
			accountConfigs = []bot.AccountConfig{single}
		} else {
			// Tentar formato exportado do browser (cookies: "...")
			var exported struct {
				Cookies      string `json:"cookies"`
				SessionProof string `json:"sessionProof"`
			}
			if errExp := json.Unmarshal(content, &exported); errExp == nil && exported.Cookies != "" {
				accountConfigs = []bot.AccountConfig{{
					ID:            "acc-1",
					SessionCookie: exported.Cookies,
					SessionProof:  exported.SessionProof,
					AutoSell:      true,
					AutoBlessings: true,
					AutoDaily:     true,
				}}
			} else {
				log.Fatalf("Falha ao analisar %s: %v", configFile, err)
			}
		}
	}

	targetAcc := os.Getenv("ACCOUNT")
	var activeConfigs []bot.AccountConfig

	for _, a := range accountConfigs {
		if targetAcc != "" && a.ID != targetAcc {
			continue
		}
		if a.ID == "" {
			a.ID = fmt.Sprintf("acc-%d", len(activeConfigs)+1)
		}
		activeConfigs = append(activeConfigs, a)
	}

	if len(activeConfigs) == 0 {
		log.Fatalf("Nenhuma conta configurada para rodar.")
	}

	log.Printf("Iniciando %d conta(s) em Goroutines nativas…\n", len(activeConfigs))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	manager := &BotManager{}
	for _, cfg := range activeConfigs {
		b := bot.NewBot(cfg)
		manager.bots = append(manager.bots, b)
		go b.Run(ctx)
	}

	// 2. Iniciar Servidor Web HTTP na Porta 3001 (evita conflito com huntera-bot na 3000)
	webPort := 3001
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			webPort = v
		}
	}
	srv := server.NewServer(webPort, manager)
	go func() {
		log.Printf("[Web Server] Dashboard online em http://localhost:%d\n", webPort)
		if err := srv.Start(); err != nil {
			log.Printf("[Web Server] Encerrado: %v", err)
		}
	}()

	// 3. Monitor no Terminal com estatísticas de RAM a cada 4 segundos
	go func() {
		time.Sleep(3 * time.Second)
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				allocMB := float64(m.Alloc) / 1024.0 / 1024.0
				sysMB := float64(m.Sys) / 1024.0 / 1024.0

				statuses := manager.GetAllStatuses()
				totalGold := 0
				totalKills := 0
				huntingCount := 0

				for _, st := range statuses {
					totalGold += st.Gold
					totalKills += st.Kills
					if st.Status == "hunting" {
						huntingCount = huntingCount + 1
					}
				}

				fmt.Print("\033[H\033[2J") // Limpar tela do terminal
				fmt.Println("================================================================================")
				fmt.Printf("   ⚔  TIBIDLE BOT CONSOLE — RAM: %.2f MB (Sys: %.2f MB) | Goroutines: %d\n", allocMB, sysMB, runtime.NumGoroutine())
				fmt.Printf("   📊 Contas Ativas: %d/%d Caçando | Ouro Total: %s | Total Kills: %d\n", huntingCount, len(statuses), formatGold(totalGold), totalKills)
				fmt.Printf("   🌐 Dashboard Web: http://localhost:%d\n", webPort)
				fmt.Println("================================================================================")

				for _, st := range statuses {
					hpPct := 0
					if st.MaxHP > 0 {
						hpPct = (st.HP * 100) / st.MaxHP
					}
					manaPct := 0
					if st.MaxMana > 0 {
						manaPct = (st.Mana * 100) / st.MaxMana
					}

					fmt.Printf("▸ [%s] %s (Lv. %d %s) | Status: %s\n", st.ID, st.Name, st.Level, st.Vocation, st.Status)
					fmt.Printf("  HP: %d/%d (%d%%) | Mana: %d/%d (%d%%) | Ouro: %s | Kills: %d\n", st.HP, st.MaxHP, hpPct, st.Mana, st.MaxMana, manaPct, formatGold(st.Gold), st.Kills)
					if st.CurrentHunt != "" {
						fmt.Printf("  Caçada: %s | EXP/H: %.0f | Duração: %ds\n", st.CurrentHunt, st.ExpPerHour, st.DurationSec)
					}
					fmt.Println("--------------------------------------------------------------------------------")
				}
			}
		}
	}()

	// 4. Capturar sinais para desligamento seguro
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	log.Println("\nSinal de encerramento recebido. Desligando bots...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("Tibidle Bot finalizado com sucesso.")
}
