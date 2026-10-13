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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"tibidle-bot/pkg/auth"
)

type remoteAccount struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type remoteStatus struct {
	Accounts  []remoteAccount `json:"accounts"`
	TunnelURL string          `json:"tunnelUrl"`
	UpdatedAt int64           `json:"updatedAt"`
}

type localAccount struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	SessionCookie string `json:"sessionCookie"`
}

type ticketPayload struct {
	AccountID string `json:"accountId"`
	Ticket    string `json:"ticket"`
	WSURL     string `json:"wsUrl"`
}

var (
	accountsPath  = "accounts.json"
	mu            sync.Mutex
	genLock       sync.Mutex
	lastStatus    remoteStatus
	lastTransport string
	lastPollAt    time.Time
	waitStreak    = map[string]int{}
	lastPushAt    = map[string]time.Time{}
)

const (
	minPushInterval = 45 * time.Second
	requiredStreak  = 2
)

func main() {
	poll := flag.Duration("poll", 15*time.Second, "Intervalo de poll do status do bot")
	flag.StringVar(&accountsPath, "accounts", "accounts.json", "accounts.json local (cookies/senha)")
	sshTarget := flag.String("ssh", "ubuntu@129.153.157.17", "Alvo SSH da VPS")
	sshKey := flag.String("key", "", "Chave SSH (vazia = procura padrão)")
	direct := flag.String("direct", "", "URL direta do bot na VPS (ex: http://129.153.157.17:3001), se porta liberada")
	statusURL := flag.String("status-url", "https://raw.githubusercontent.com/kbelludoo/tibidle-bot/main/docs/status.json", "status.json no GitHub (descoberta do tunnel)")
	listen := flag.String("listen", "127.0.0.1:3901", "Endpoint local GET /ticket")
	once := flag.Bool("once", false, "Um único poll e sai")
	flag.Parse()

	keyPath := *sshKey
	if keyPath == "" {
		for _, c := range []string{
			`C:\Users\kbell\.ssh\ssh-key-2026-10-06.key`,
			"ssh-key-2026-10-06.key",
		} {
			if _, err := os.Stat(c); err == nil {
				keyPath = c
				break
			}
		}
	}

	log.SetFlags(log.Ltime)
	if lf, err := os.OpenFile("ticketd.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, lf))
		defer lf.Close()
	}
	log.Printf("[ticketd] relay iniciado | poll=%s | ssh=%s | status=%s", *poll, *sshTarget, *statusURL)

	go serveLocal(*listen)

	for {
		if err := pollOnce(*sshTarget, keyPath, *direct, *statusURL); err != nil {
			log.Printf("[ticketd] poll falhou: %v", err)
		}
		if *once {
			return
		}
		time.Sleep(*poll)
	}
}

func fetchStatus(sshTarget, keyPath, direct, statusURL string) (remoteStatus, string, error) {
	// 1. Direto (se porta exposta — normalmente bloqueada pela VPS)
	if direct != "" {
		var st remoteStatus
		if err := httpGetJSON(strings.TrimRight(direct, "/")+"/api/status", &st); err == nil {
			return st, "direct", nil
		}
	}

	// 2. SSH (mais confiável: lê status interno da VPS sem tunnel)
	if st, err := sshStatus(sshTarget, keyPath); err == nil && len(st.Accounts) > 0 {
		return st, "ssh", nil
	}

	// 3. Tunnel via status.json do GitHub
	var meta remoteStatus
	if err := httpGetJSON(statusURL, &meta); err == nil && meta.TunnelURL != "" {
		var st remoteStatus
		if err := httpGetJSON(strings.TrimRight(meta.TunnelURL, "/")+"/api/status", &st); err == nil && len(st.Accounts) > 0 {
			return st, "tunnel", nil
		}
	}

	return remoteStatus{}, "", fmt.Errorf("nenhum transporte de status disponível")
}

func sshStatus(sshTarget, keyPath string) (remoteStatus, error) {
	out, err := sshRun(sshTarget, keyPath, "", `curl -s -m 5 http://127.0.0.1:3001/api/status`)
	if err != nil {
		return remoteStatus{}, err
	}
	var st remoteStatus
	if err := json.Unmarshal(out, &st); err != nil {
		return remoteStatus{}, fmt.Errorf("status inválido via ssh: %w", err)
	}
	return st, nil
}

func pollOnce(sshTarget, keyPath, direct, statusURL string) error {
	st, transport, err := fetchStatus(sshTarget, keyPath, direct, statusURL)
	if err != nil {
		return err
	}

	mu.Lock()
	if transport != lastTransport {
		log.Printf("[ticketd] transporte de status: %s -> %s", lastTransport, transport)
	}
	lastStatus = st
	lastTransport = transport
	lastPollAt = time.Now()
	mu.Unlock()

	var pending []remoteAccount
	for _, acc := range st.Accounts {
		if needsTicket(acc.Status) {
			waitStreak[acc.ID]++
			if waitStreak[acc.ID] >= requiredStreak && time.Since(lastPushAt[acc.ID]) >= minPushInterval {
				pending = append(pending, acc)
			}
		} else {
			waitStreak[acc.ID] = 0
		}
	}
	if len(pending) == 0 {
		return nil
	}

	for _, acc := range pending {
		if err := pushTicketFor(acc, st, sshTarget, keyPath, statusURL); err != nil {
			log.Printf("[ticketd] falha ao enviar ticket para %s: %v", acc.ID, err)
		} else {
			lastPushAt[acc.ID] = time.Now()
		}
	}
	return nil
}

// needsTicket detecta o estado em que o bot está parado aguardando push.
func needsTicket(status string) bool {
	return strings.Contains(strings.ToLower(status), "aguardando ticket")
}

func pushTicketFor(acc remoteAccount, st remoteStatus, sshTarget, keyPath, statusURL string) error {
	genLock.Lock()
	defer genLock.Unlock()

	cookies, err := cookiesFor(acc.ID)
	if err != nil {
		return err
	}

	log.Printf("[ticketd] bot em '%s' — abrindo navegador local para gerar ticket...", acc.Status)
	res, err := auth.FetchTicketViaBrowser(cookies)
	if err != nil {
		return fmt.Errorf("navegador: %w", err)
	}

	payload, _ := json.Marshal(ticketPayload{
		AccountID: acc.ID,
		Ticket:    res.Ticket,
		WSURL:     res.WSURL,
	})

	if err := pushViaSSH(sshTarget, keyPath, payload); err == nil {
		log.Printf("[ticketd] ✔ ticket de %s enviado via SSH", acc.ID)
		return nil
	} else {
		log.Printf("[ticketd] push via SSH falhou: %v", err)
	}
	if tunnel := tunnelFor(st, statusURL); tunnel != "" {
		target := strings.TrimRight(tunnel, "/") + "/api/ticket"
		client := &http.Client{Timeout: 10 * time.Second}
		if resp, err := client.Post(target, "application/json", bytes.NewReader(payload)); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				log.Printf("[ticketd] ✔ ticket de %s enviado via tunnel %s", acc.ID, tunnel)
				return nil
			}
			return fmt.Errorf("tunnel HTTP %d", resp.StatusCode)
		}
	}
	return fmt.Errorf("todos os transports de push falharam")
}

func pushViaSSH(sshTarget, keyPath string, payload []byte) error {
	_, err := sshRun(sshTarget, keyPath, string(payload),
		`curl -s -m 10 -X POST http://127.0.0.1:3001/api/ticket -H 'Content-Type: application/json' -d @-`)
	return err
}

func tunnelFor(st remoteStatus, statusURL string) string {
	if st.TunnelURL != "" {
		return st.TunnelURL
	}
	var meta remoteStatus
	if err := httpGetJSON(statusURL, &meta); err == nil {
		return meta.TunnelURL
	}
	return ""
}

func sshRun(sshTarget, keyPath, stdin, remoteCmd string) ([]byte, error) {
	args := []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10"}
	if keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, sshTarget, remoteCmd)
	cmd := exec.Command("ssh", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ssh: %v (%s)", err, strings.TrimSpace(out.String()))
	}
	return out.Bytes(), nil
}

// cookiesFor lê os cookies da conta no accounts.json local; renova via senha
// se a sessão local estiver morta (login não exige captcha).
func cookiesFor(accountID string) (string, error) {
	path := resolveAccountsPath()
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("ler %s: %w", path, err)
	}
	var accs []localAccount
	if err := json.Unmarshal(content, &accs); err != nil || len(accs) == 0 {
		return "", fmt.Errorf("%s sem contas válidas", path)
	}

	pick := -1
	for i, a := range accs {
		if a.ID == accountID {
			pick = i
			break
		}
	}
	if pick == -1 {
		// conta vazia ou status anonimizado ("acc1") — usa a primeira
		pick = 0
	}

	acc := accs[pick]
	if acc.SessionCookie == "" && acc.Password == "" {
		return "", fmt.Errorf("conta %s sem cookie nem senha", acc.ID)
	}
	needLogin := acc.SessionCookie == ""
	if acc.SessionCookie != "" && acc.Password != "" {
		if _, _, err := validateCookie(acc.SessionCookie); err != nil {
			log.Printf("[ticketd] sessão local expirou/inválida (%v) — renovando via login para %s...", err, acc.ID)
			needLogin = true
		}
	}
	if needLogin {
		if acc.Password == "" || acc.Email == "" {
			return "", fmt.Errorf("conta %s sem senha para renovar sessão", acc.ID)
		}
		cookies, _, lerr := login(acc.Email, acc.Password)
		if lerr != nil {
			return "", fmt.Errorf("renovação de sessão falhou: %w", lerr)
		}
		acc.SessionCookie = cookies
		accs[pick].SessionCookie = cookies
		if out, merr := json.MarshalIndent(accs, "", "  "); merr == nil {
			_ = os.WriteFile(path, out, 0644)
		}
		log.Printf("[ticketd] sessão renovada e salva em %s", path)
	}
	return acc.SessionCookie, nil
}

func validateCookie(cookies string) (string, int, error) {
	req, _ := http.NewRequest("GET", "https://play.tibidle.com/auth/me", nil)
	req.Header.Set("Cookie", cookies)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return string(body), resp.StatusCode, fmt.Errorf("auth/me HTTP %d", resp.StatusCode)
	}
	return string(body), resp.StatusCode, nil
}

func login(email, password string) (string, int, error) {
	body, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req, _ := http.NewRequest("POST", "https://play.tibidle.com/auth/password/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://play.tibidle.com")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, b)
	}
	var parts []string
	for _, c := range resp.Header.Values("Set-Cookie") {
		parts = append(parts, strings.TrimSpace(strings.Split(c, ";")[0]))
	}
	return strings.Join(parts, "; "), resp.StatusCode, nil
}

func httpGetJSON(url string, v any) error {
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: HTTP %d", url, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func resolveAccountsPath() string {
	if _, err := os.Stat(accountsPath); err == nil {
		return accountsPath
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "accounts.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return accountsPath
}

// serveLocal expõe GET /ticket para push manual e GET /status para debug.
func serveLocal(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ticket", func(w http.ResponseWriter, r *http.Request) {
		cookies, err := cookiesFor("")
		if err != nil {
			http.Error(w, err.Error(), http.StatusPreconditionFailed)
			return
		}
		genLock.Lock()
		defer genLock.Unlock()
		res, err := auth.FetchTicketViaBrowser(cookies)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"ticket": res.Ticket,
			"wsUrl":  res.WSURL,
		})
	})
	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"transport":  lastTransport,
			"lastPollAt": lastPollAt,
			"accounts":   lastStatus.Accounts,
		})
	})
	log.Printf("[ticketd] endpoint local em http://%s (/ticket, /status)", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Printf("[ticketd] servidor local parou: %v", err)
	}
}
