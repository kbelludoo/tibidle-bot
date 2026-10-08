package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tibidle-bot/pkg/auth"
)

const (
	BaseURL   = "https://play.tibidle.com"
	UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

type AccountInfo struct {
	AccountID string `json:"accountId"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	WorldID   string `json:"worldId"`
}

type SessionClient struct {
	httpClient *http.Client
}

func NewSessionClient() *SessionClient {
	return &SessionClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// ValidateSession verifies if current cookies are active
func (s *SessionClient) ValidateSession(cookies string) (*AccountInfo, error) {
	req, err := http.NewRequest("GET", BaseURL+"/auth/me", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Cookie", cookies)
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", BaseURL+"/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error on auth/me: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("session invalid (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var acc AccountInfo
	if err := json.NewDecoder(resp.Body).Decode(&acc); err != nil {
		return nil, fmt.Errorf("failed to decode account info: %w", err)
	}

	return &acc, nil
}

// LoginWithEmailPassword performs password login
func (s *SessionClient) LoginWithEmailPassword(email, password string) (string, *AccountInfo, error) {
	bodyData, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": password,
	})

	req, err := http.NewRequest("POST", BaseURL+"/auth/password/login", bytes.NewReader(bodyData))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Origin", BaseURL)
	req.Header.Set("Referer", BaseURL+"/")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("login failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("login rejected (HTTP %d): %s", resp.StatusCode, string(b))
	}

	var cookieParts []string
	for _, c := range resp.Header.Values("Set-Cookie") {
		parts := strings.Split(c, ";")
		cookieParts = append(cookieParts, strings.TrimSpace(parts[0]))
	}
	cookieStr := strings.Join(cookieParts, "; ")

	var res struct {
		Account AccountInfo `json:"account"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	return cookieStr, &res.Account, nil
}

// GetGameTicket obtains ticket via direct API or via the 4-second headless browser helper
func (s *SessionClient) GetGameTicket(cookies string, sessionProof string) (string, string, error) {
	// 1. Try direct HTTP route first (if sessionProof or cached token works)
	ticket, wsURL, err := s.tryDirectTicket(cookies, sessionProof)
	if err == nil && ticket != "" {
		return ticket, wsURL, nil
	}

	// 2. If captcha or browser challenge is required, use lightweight CDP helper
	ticket, err = auth.FetchTicketViaBrowser(cookies)
	if err != nil {
		return "", "", fmt.Errorf("failed to obtain ticket: %w", err)
	}

	return ticket, "wss://play.tibidle.com/", nil
}

func (s *SessionClient) tryDirectTicket(cookies string, sessionProof string) (string, string, error) {
	// A. Nonce
	nonceReq, err := http.NewRequest("POST", BaseURL+"/auth/world-nonce", nil)
	if err != nil {
		return "", "", err
	}
	nonceReq.Header.Set("Cookie", cookies)
	nonceReq.Header.Set("Origin", BaseURL)
	nonceReq.Header.Set("Referer", BaseURL+"/")
	nonceReq.Header.Set("User-Agent", UserAgent)
	nonceReq.Header.Set("sec-ch-ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	nonceReq.Header.Set("sec-ch-ua-mobile", "?0")
	nonceReq.Header.Set("sec-ch-ua-platform", `"Windows"`)

	nonceResp, err := s.httpClient.Do(nonceReq)
	if err != nil {
		return "", "", err
	}
	defer nonceResp.Body.Close()

	if nonceResp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("world-nonce HTTP %d", nonceResp.StatusCode)
	}

	var nonceData struct {
		Nonce string `json:"nonce"`
	}
	if err := json.NewDecoder(nonceResp.Body).Decode(&nonceData); err != nil || nonceData.Nonce == "" {
		return "", "", fmt.Errorf("invalid nonce response")
	}

	// B. Spoofed Laudo + World Ticket
	laudoPayload := map[string]any{
		"laudo": map[string]any{
			"v":      1,
			"nonce":  nonceData.Nonce,
			"versao": "detector-2",
			"t":      time.Now().UnixMilli(),
			"sinais": map[string]string{
				"ua_headless":           "nao",
				"webdriver":             "nao",
				"webdriver_adulterado":  "nao",
				"brands_vazio":          "nao",
				"brands_sem_fornecedor": "nao",
				"websocket_substituido": "nao",
				"fetch_substituido":     "nao",
				"global_automacao":      "nao",
				"widevine_ausente":      "nao",
				"renderer_software":     "nao",
				"worker_divergente":     "nao",
				"webview_embutida":      "nao",
				"ponte_nativa":          "nao",
				"visibilidade_forjada":  "nao",
			},
		},
	}
	payloadBytes, _ := json.Marshal(laudoPayload)

	ticketReq, err := http.NewRequest("POST", BaseURL+"/auth/world-ticket", bytes.NewReader(payloadBytes))
	if err != nil {
		return "", "", err
	}
	ticketReq.Header.Set("Content-Type", "application/json")
	ticketReq.Header.Set("Cookie", cookies)
	ticketReq.Header.Set("Origin", BaseURL)
	ticketReq.Header.Set("Referer", BaseURL+"/")
	ticketReq.Header.Set("User-Agent", UserAgent)
	ticketReq.Header.Set("sec-ch-ua", `"Google Chrome";v="131", "Chromium";v="131", "Not_A Brand";v="24"`)
	ticketReq.Header.Set("sec-ch-ua-mobile", "?0")
	ticketReq.Header.Set("sec-ch-ua-platform", `"Windows"`)
	if sessionProof != "" {
		ticketReq.Header.Set("X-Session-Proof", sessionProof)
	}

	ticketResp, err := s.httpClient.Do(ticketReq)
	if err != nil {
		return "", "", err
	}
	defer ticketResp.Body.Close()

	if ticketResp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("world-ticket HTTP %d", ticketResp.StatusCode)
	}

	var ticketData struct {
		Ticket string `json:"ticket"`
		WSURL  string `json:"wsUrl"`
	}
	if err := json.NewDecoder(ticketResp.Body).Decode(&ticketData); err != nil || ticketData.Ticket == "" {
		return "", "", fmt.Errorf("empty ticket returned")
	}

	return ticketData.Ticket, ticketData.WSURL, nil
}
