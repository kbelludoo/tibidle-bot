package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
			`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
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
		home, _ := os.UserHomeDir()
		candidates := []string{
			"/usr/bin/google-chrome-stable",
			"/usr/bin/google-chrome",
			filepath.Join(home, ".cache/puppeteer/chrome/linux-131.0.6778.204/chrome-linux64/chrome"),
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
			"/snap/bin/chromium",
		}

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

// cdpClient: cliente CDP com correlação id->resposta (request/notify) e eventos.
type cdpClient struct {
	conn    *websocket.Conn
	writeMu sync.Mutex

	mu      sync.Mutex
	nextID  int
	pending map[int]chan json.RawMessage

	onEvent func(method string, params json.RawMessage)
	closed  bool
}

func newCDPClient(conn *websocket.Conn, onEvent func(string, json.RawMessage)) *cdpClient {
	c := &cdpClient{
		conn:    conn,
		pending: make(map[int]chan json.RawMessage),
		onEvent: onEvent,
	}
	go c.readLoop()
	return c
}

func (c *cdpClient) readLoop() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			c.mu.Lock()
			c.closed = true
			for id, ch := range c.pending {
				close(ch)
				delete(c.pending, id)
			}
			c.mu.Unlock()
			return
		}
		var env struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
		}
		if json.Unmarshal(msg, &env) != nil {
			continue
		}
		if env.ID != nil && env.Method == "" {
			c.mu.Lock()
			ch := c.pending[*env.ID]
			delete(c.pending, *env.ID)
			c.mu.Unlock()
			if ch != nil {
				if len(env.Result) > 0 {
					ch <- env.Result
				} else {
					ch <- env.Params
				}
			}
			continue
		}
		if env.Method != "" && c.onEvent != nil {
			c.onEvent(env.Method, env.Params)
		}
	}
}

func (c *cdpClient) notify(method string, params map[string]any) {
	b, _ := json.Marshal(map[string]any{"id": 0, "method": method, "params": params})
	c.writeMu.Lock()
	_ = c.conn.WriteMessage(websocket.TextMessage, b)
	c.writeMu.Unlock()
}

func (c *cdpClient) request(method string, params map[string]any) (json.RawMessage, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, fmt.Errorf("cdp fechado")
	}
	c.nextID++
	id := c.nextID
	ch := make(chan json.RawMessage, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	b, _ := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	c.writeMu.Lock()
	err := c.conn.WriteMessage(websocket.TextMessage, b)
	c.writeMu.Unlock()
	if err != nil {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, err
	}

	select {
	case res, ok := <-ch:
		if !ok || res == nil {
			return nil, fmt.Errorf("cdp fechado aguardando %s", method)
		}
		return res, nil
	case <-time.After(5 * time.Second):
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("timeout cdp em %s", method)
	}
}

func (c *cdpClient) evalJS(expr string) (string, error) {
	raw, err := c.request("Runtime.evaluate", map[string]any{
		"expression":    expr,
		"returnByValue": true,
	})
	if err != nil {
		return "", err
	}
	var out struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", err
	}
	if out.ExceptionDetails != nil {
		return "", fmt.Errorf("exceção JS: %s", out.ExceptionDetails.Text)
	}
	if len(out.Result.Value) == 0 || string(out.Result.Value) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(out.Result.Value, &s); err != nil {
		return "", nil
	}
	return s, nil
}

func (c *cdpClient) clickXY(x, y float64) {
	c.notify("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y})
	c.notify("Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1})
	c.notify("Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1})
}

// exprAlvo: varre documento + shadow roots; devolve JSON {ifr:[retangulos de
// iframes visiveis], hosts:[tag#id.cls:retangulo de elementos do desafio]}.
const exprAlvo = `(() => {
	const ifr = [], hosts = [], seen = new Set();
	const cls = el => ((el.id ? '#' + el.id : '') + (typeof el.className === 'string' && el.className ? '.' + el.className.trim().replace(/\s+/g, '.') : '')).slice(0, 50);
	const walk = (root, d) => {
		if (d > 14) return;
		for (const el of root.querySelectorAll('*')) {
			const r = el.getBoundingClientRect();
			if (el.tagName === 'IFRAME' && r.width >= 40 && r.height >= 15) {
				ifr.push(Math.round(r.x) + ',' + Math.round(r.y) + ',' + Math.round(r.width) + ',' + Math.round(r.height));
			} else if (r.width > 40 && r.height > 15) {
				const s = cls(el) + ' ' + (el.getAttribute('aria-label') || '');
				if (/turnstile|challenge|captcha|cf-/i.test(s)) hosts.push(cls(el) + ':' + Math.round(r.x) + ',' + Math.round(r.y) + ',' + Math.round(r.width) + ',' + Math.round(r.height));
			}
			if (el.shadowRoot && !seen.has(el.shadowRoot)) { seen.add(el.shadowRoot); walk(el.shadowRoot, d + 1); }
		}
	};
	walk(document, 0);
	return JSON.stringify({ ifr: ifr, hosts: hosts.slice(0, 8) });
})()`

// exprAcharTexto: fallback — acha o container do texto "not a robot" (desafio
// gerenciado sem iframe visível) e devolve "texto:x,y,w,h".
const exprAcharTexto = `(() => {
	const walk = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
	let node;
	while ((node = walk.nextNode())) {
		if (/not a robot|não é um robô|nao e um robo/i.test(node.nodeValue || '')) {
			let el = node.parentElement;
			for (let i = 0; el && i < 6; i++, el = el.parentElement) {
				const r = el.getBoundingClientRect();
				if (r.width > 80 && r.height > 20 && r.top >= 0 && r.bottom <= window.innerHeight + 50) {
					return 'texto:' + Math.round(r.x) + ',' + Math.round(r.y) + ',' + Math.round(r.width) + ',' + Math.round(r.height);
				}
			}
		}
	}
	return '';
})()`

// alvoDesafio: prioridade iframe (piercing shadow DOM) > host do desafio > texto.
func alvoDesafio(c *cdpClient) (x, y, w, h float64, metodo string, ok bool) {
	if raw, err := c.evalJS(exprAlvo); err == nil && raw != "" {
		var t struct {
			Ifr   []string `json:"ifr"`
			Hosts []string `json:"hosts"`
		}
		if json.Unmarshal([]byte(raw), &t) == nil {
			if len(t.Ifr) > 0 {
				if n, _ := fmt.Sscanf(t.Ifr[0], "%f,%f,%f,%f", &x, &y, &w, &h); n == 4 {
					return x, y, w, h, "iframe", true
				}
			}
			for _, hs := range t.Hosts {
				cord := hs
				if i := strings.IndexByte(hs, ':'); i >= 0 {
					cord = hs[i+1:]
				}
				if n, _ := fmt.Sscanf(cord, "%f,%f,%f,%f", &x, &y, &w, &h); n == 4 {
					return x, y, w, h, "host", true
				}
			}
		}
	}
	if v, err := c.evalJS(exprAcharTexto); err == nil && v != "" {
		cord := v
		if i := strings.IndexByte(v, ':'); i >= 0 {
			cord = v[i+1:]
		}
		if n, _ := fmt.Sscanf(cord, "%f,%f,%f,%f", &x, &y, &w, &h); n == 4 {
			return x, y, w, h, "texto", true
		}
	}
	return 0, 0, 0, 0, "", false
}

// FetchTicketViaBrowser launches a browser, resolves the challenge and captures a live ticket.
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
		"--window-size=960,720",
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
	var chromeOut bytes.Buffer
	cmd.Stdout = &chromeOut
	cmd.Stderr = &chromeOut
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start auth browser (%s): %w", browserPath, err)
	}

	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()

	var wsURL string
	startTime := time.Now()
	for time.Since(startTime) < 20*time.Second {
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
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return nil, fmt.Errorf("could not connect to browser CDP (%s) within timeout [chrome: %s]", browserPath, strings.TrimSpace(chromeOut.String()))
	}

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial CDP websocket: %w", err)
	}
	defer conn.Close()

	ticketChan := make(chan TicketResult, 1)

	cdp := newCDPClient(conn, func(method string, params json.RawMessage) {
		if method != "Runtime.consoleAPICalled" {
			return
		}
		var ev struct {
			Args []struct {
				Type  string `json:"type"`
				Value string `json:"value"`
			} `json:"args"`
		}
		if json.Unmarshal(params, &ev) != nil {
			return
		}
		for _, arg := range ev.Args {
			if strings.Contains(arg.Value, "AUTO_CLICKED") || strings.Contains(arg.Value, "BLOCKED_BROWSER_WS") {
				log.Printf("[Browser Console] %s", arg.Value)
			}
			if strings.HasPrefix(arg.Value, "INTERCEPTED_TICKET_DATA:") {
				jsonPayload := strings.TrimPrefix(arg.Value, "INTERCEPTED_TICKET_DATA:")
				var res TicketResult
				if err := json.Unmarshal([]byte(jsonPayload), &res); err == nil && res.Ticket != "" {
					select {
					case ticketChan <- res:
					default:
					}
				}
			}
		}
	})

	cdp.notify("Network.enable", nil)
	cdp.notify("Page.enable", nil)
	cdp.notify("Runtime.enable", nil)

	// Inject early hook to capture ticket and BLOCK browser WebSocket so ticket stays UNUSED
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
			if (btn && !btn.disabled) {
				console.log("AUTO_CLICKED_LOBBY_BTN");
				btn.click();
			}
		}, 1000);
	`

	cdp.notify("Page.addScriptToEvaluateOnNewDocument", map[string]any{
		"source": hookScript,
	})

	cookieParts := strings.Split(cookieString, ";")
	for _, part := range cookieParts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			cdp.notify("Network.setCookie", map[string]any{
				"name":     kv[0],
				"value":    kv[1],
				"url":      "https://play.tibidle.com",
				"domain":   ".tibidle.com",
				"path":     "/",
				"secure":   true,
				"httpOnly": true,
			})
		}
	}

	time.Sleep(200 * time.Millisecond)
	log.Println("[AuthHelper] Injected session cookies, navigating to play.tibidle.com...")
	cdp.notify("Page.navigate", map[string]any{
		"url": "https://play.tibidle.com/",
	})

	// Auto-clique no desafio interativo do Turnstile (iframe challenges.cloudflare.com)
	stopClicks := make(chan struct{})
	defer close(stopClicks)
	go func() {
		ticker := time.NewTicker(1500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopClicks:
				return
			case <-ticker.C:
				x, y, w, h, metodo, ok := alvoDesafio(cdp)
				if !ok {
					continue
				}
				log.Printf("[AuthHelper] desafio Turnstile visivel via %s %d,%d,%d,%d — clicando", metodo, int(x), int(y), int(w), int(h))
				dx := 30.0
				if w < 70 {
					dx = w / 2
				}
				cdp.clickXY(x+dx, y+h/2)
			}
		}
	}()

	select {
	case res := <-ticketChan:
		log.Printf("[AuthHelper] Fresh, UNUSED game ticket captured (WS: %s)! Shutting down browser.\n", res.WSURL)
		return &res, nil
	case <-time.After(90 * time.Second):
		dump, derr := cdp.evalJS(`JSON.stringify({url:location.href,title:document.title,text:document.body?document.body.innerText.slice(0,300):''})`)
		if derr != nil {
			dump = "evalJS falhou: " + derr.Error()
		}
		alvos, _ := cdp.evalJS(exprAlvo)
		return nil, fmt.Errorf("ticket capture timed out after 90s [page=%s | alvos=%s]", dump, alvos)
	}
}
