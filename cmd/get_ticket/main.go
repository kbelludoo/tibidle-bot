package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"tibidle-bot/pkg/auth"
)

func main() {
	pushURL := flag.String("push", "", "URL do servidor HTTP do bot (ex: http://localhost:3001)")
	sshTarget := flag.String("ssh", "ubuntu@129.153.157.17", "Destino SSH do bot na VPS (ex: ubuntu@129.153.157.17)")
	sshKey := flag.String("key", `C:\Users\kbell\.ssh\ssh-key-2026-10-06.key`, "Chave SSH para conexão direta")
	noPush := flag.Bool("no-push", false, "Apenas gerar o ticket e exibir na tela, sem enviar")
	cookiesFlag := flag.String("cookies", "", "Cookies da sessão (ou lê de accounts.json)")
	flag.Parse()

	cookies := *cookiesFlag
	if cookies == "" {
		if content, err := os.ReadFile("accounts.json"); err == nil {
			var accs []struct {
				SessionCookie string `json:"sessionCookie"`
			}
			if err := json.Unmarshal(content, &accs); err == nil && len(accs) > 0 && accs[0].SessionCookie != "" {
				cookies = accs[0].SessionCookie
			}
		}
	}

	if cookies == "" {
		cookies = "did=14d2df158e5f0f5e3df5bdb755bec55100eee1ba2937fc585a65e5c57e8a4a65; sid=9189720e9b70b76e8f2cbf07fdc05ed127b343f7313d13619bf65b48a42358b7"
	}

	log.Println("[TicketGenerator] Abrindo navegador local para resolver desafio e obter ticket...")
	ticket, err := auth.FetchTicketViaBrowser(cookies)
	if err != nil {
		log.Fatalf("Erro ao capturar ticket: %v", err)
	}

	fmt.Println("\n================================================================================")
	fmt.Printf("FRESH_TICKET:%s\n", ticket)
	fmt.Println("================================================================================")

	if *noPush {
		return
	}

	// 1. Enviar via HTTP direto se especificado
	if *pushURL != "" {
		target := strings.TrimRight(*pushURL, "/") + "/api/ticket"
		log.Printf("[TicketGenerator] Enviando ticket via HTTP para %s ...", target)

		payload, _ := json.Marshal(map[string]string{
			"ticket": ticket,
		})
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Post(target, "application/json", bytes.NewReader(payload))
		if err != nil {
			log.Printf("Aviso: Falha ao enviar via HTTP direto (%v)", err)
		} else {
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusOK {
				log.Printf("✔ SUCESSO HTTP! Ticket aceito pelo bot: %s", string(body))
				return
			}
		}
	}

	// 2. Enviar via SSH direto para a VPS
	if *sshTarget != "" {
		log.Printf("[TicketGenerator] Enviando ticket com segurança via SSH para %s ...", *sshTarget)
		remoteCmd := fmt.Sprintf(`curl -s -X POST http://127.0.0.1:3001/api/ticket -H 'Content-Type: application/json' -d '{"ticket":"%s"}'`, ticket)

		var cmd *exec.Cmd
		if *sshKey != "" {
			cmd = exec.Command("ssh", "-i", *sshKey, "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", *sshTarget, remoteCmd)
		} else {
			cmd = exec.Command("ssh", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", *sshTarget, remoteCmd)
		}

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatalf("Falha ao enviar ticket via SSH: %v, saída: %s", err, string(out))
		}
		log.Printf("✔ SUCESSO SSH! Bot respondeu: %s", string(out))
	}
}
