package auth

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

type CDPTarget struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

type TicketResult struct {
	Ticket string `json:"ticket"`
	WSURL  string `json:"wsUrl"`
}

// FindBrowserExecutable finds Chrome or Edge depending on OS
func FindBrowserExecutable() string {
	if os.Getenv("NO_BROWSER") == "1" {
		return ""
	}

	if env := os.Getenv("CHROME_PATH"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}

	if runtime.GOOS == "windows" {
		candidates := []string{
			`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	} else {
		// Linux VPS
		home, _ := os.UserHomeDir()
		candidates := []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			filepath.Join(home, ".cache/puppeteer/chrome/linux-131.0.6778.204/chrome-linux64/chrome"),
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}

		// Also check any puppeteer chrome installation dynamically
		matches, _ := filepath.Glob(filepath.Join(home, ".cache/puppeteer/chrome/*/chrome-linux64/chrome"))
		candidates = append(matches, candidates...)

		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}

// FetchTicketViaBrowser launches a browser for ~4 seconds, extracts a live WebSocket ticket,
// and terminates the browser immediately to keep VPS memory footprint minimal (~15MB RAM).
func FetchTicketViaBrowser(cookieString string) (*TicketResult, error) {
	browserPath := FindBrowserExecutable()
	if browserPath == "" {
		return nil, fmt.Errorf("no browser executable found (install chromium or google-chrome)")
	}

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("tibidle-auth-%d", time.Now().UnixNano()))
	defer os.RemoveAll(tempDir)

	port := 9226
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		fmt.Sprintf("--user-data-dir=%s", tempDir),
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-default-apps",
		"--disable-extensions",
		"--disable-sync",
		"--window-position=-3000,-3000",
		"--window-size=800,600",
	}

	if runtime.GOOS != "windows" {
		args = append(args,
			"--no-sandbox",
			"--disable-setuid-sandbox",
			"--disable-gpu",
			"--disable-dev-shm-usage",
			"--no-zygote",
		)
		if os.Getenv("DISPLAY") == "" {
			args = append(args, "--headless=new")
		}
	}

	cmd := exec.Command(browserPath, args...)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start auth browser (%s): %w", browserPath, err)
	}

	// Always ensure the browser process is killed when this function returns
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	// Wait for CDP port to respond (up to 8s)
	var wsURL string
	startTime := time.Now()
	for time.Since(startTime) < 8*time.Second {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/json/list", port))
		if err == nil {
			var targets []CDPTarget
			_ = json.NewDecoder(resp.Body).Decode(&targets)
			resp.Body.Close()
			for _, t := range targets {
				if t.Type == "page" {
					wsURL = t.WebSocketDebuggerURL
					break
				}
			}
			if wsURL != "" {
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}

	if wsURL == "" {
		resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/json/new?https://play.tibidle.com/", port), "", nil)
		if err == nil {
			var t CDPTarget
			_ = json.NewDecoder(resp.Body).Decode(&t)
			resp.Body.Close()
			wsURL = t.WebSocketDebuggerURL
		}
	}

	if wsURL == "" {
		return nil, fmt.Errorf("could not connect to browser CDP (%s) within timeout", browserPath)
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial CDP websocket: %w", err)
	}
	defer conn.Close()

	msgID := 1
	sendCDP := func(method string, params map[string]any) {
		id := msgID
		msgID++
		b, _ := json.Marshal(map[string]any{
			"id":     id,
			"method": method,
			"params": params,
		})
		_ = conn.WriteMessage(websocket.TextMessage, b)
	}

	sendCDP("Network.enable", nil)
	sendCDP("Page.enable", nil)
	sendCDP("Runtime.enable", nil)

	// Inject early hook to capture ticket and BLOCK browser WebSocket so ticket stays 100% UNUSED!
	hookScript := `
		const originalFetch = window.fetch;
		window.fetch = async function(...args) {
			const res = await originalFetch.apply(this, args);
			try {
				const url = typeof args[0] === 'string' ? args[0] : (args[0] && args[0].url);
				if (url && url.includes('/auth/world-ticket')) {
					const clone = res.clone();
					clone.json().then(data => {
						if (data && data.ticket) {
							console.log("INTERCEPTED_TICKET_DATA:" + JSON.stringify(data));
						}
					}).catch(() => {});
				}
			} catch(e) {}
			return res;
		};

		class DummyWebSocket extends EventTarget {
			constructor(url, protocols) {
				super();
				this.url = url;
				this.readyState = 0;
				this.bufferedAmount = 0;
				this.extensions = '';
				this.protocol = '';
				this.binaryType = 'blob';
				console.log("BLOCKED_BROWSER_WS_FOR_FRESH_TICKET:" + url);
			}
			send(data) {}
			close(code, reason) {
				this.readyState = 3;
			}
		}
		window.WebSocket = DummyWebSocket;

		// Automatically click "ENTER THE GAME" button when party lobby renders
		setInterval(() => {
			const btns = Array.from(document.querySelectorAll('button, .s-party-cta, [class*="party-cta"]'));
			const btn = btns.find(b => {
				const t = (b.innerText || '').toUpperCase();
				return t.includes('ENTER') || t.includes('ENTRAR') || t.includes('JOGO') || (b.classList && b.classList.contains('s-party-cta'));
			});
			if (btn) {
				console.log("AUTO_CLICKED_LOBBY_BTN");
				btn.click();
			}
		}, 1000);
	`

	sendCDP("Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": hookScript,
	})

	// Inject cookies
	cookieParts := strings.Split(cookieString, ";")
	for _, part := range cookieParts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			sendCDP("Network.setCookie", map[string]any{
				"name":     kv[0],
				"value":    kv[1],
				"domain":   ".tibidle.com",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			})
		}
	}

	time.Sleep(200 * time.Millisecond)
	log.Println("[AuthHelper] Injected session cookies, navigating to play.tibidle.com...")
	sendCDP("Page.navigate", map[string]any{
		"url": "https://play.tibidle.com/",
	})

	ticketChan := make(chan TicketResult, 1)

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ev struct {
				Method string `json:"method"`
				Params struct {
					Type string `json:"type"`
					Args []struct {
						Type  string `json:"type"`
						Value string `json:"value"`
					} `json:"args"`
					Text string `json:"text"`
				} `json:"params"`
			}
			if err := json.Unmarshal(msg, &ev); err != nil {
				continue
			}

			if ev.Method == "Runtime.consoleAPICalled" {
				for _, arg := range ev.Params.Args {
					if strings.Contains(arg.Value, "AUTO_CLICKED") || strings.Contains(arg.Value, "BLOCKED_BROWSER_WS") {
						log.Printf("[Browser Console] %s", arg.Value)
					}
					if strings.HasPrefix(arg.Value, "INTERCEPTED_TICKET_DATA:") {
						jsonPayload := strings.TrimPrefix(arg.Value, "INTERCEPTED_TICKET_DATA:")
						var res TicketResult
						if err := json.Unmarshal([]byte(jsonPayload), &res); err == nil && res.Ticket != "" {
							ticketChan <- res
							return
						}
					}
				}
			}
		}
	}()

	select {
	case res := <-ticketChan:
		log.Printf("[AuthHelper] Fresh, UNUSED game ticket captured (WS: %s)! Shutting down browser.\n", res.WSURL)
		return &res, nil
	case <-time.After(45 * time.Second):
		return nil, fmt.Errorf("ticket capture timed out after 45s")
	}
}
