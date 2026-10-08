const puppeteer = require('/home/ubuntu/huntera-cli/node_modules/puppeteer');

async function getTicket(cookieStr) {
  let did = '', sid = '';
  const parts = cookieStr.split(';');
  for (const p of parts) {
    const [k, v] = p.trim().split('=');
    if (k === 'did') did = v;
    if (k === 'sid') sid = v;
  }

  const browser = await puppeteer.launch({
    headless: process.env.DISPLAY ? false : 'new',
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--no-first-run'
    ]
  });

  try {
    const page = await browser.newPage();
    if (did || sid) {
      const cookies = [];
      if (did) cookies.push({ name: 'did', value: did, url: 'https://play.tibidle.com' });
      if (sid) cookies.push({ name: 'sid', value: sid, url: 'https://play.tibidle.com' });
      await page.setCookie(...cookies);
    }

    // Intercept WebSocket to prevent consumption of ticket
    await page.evaluateOnNewDocument(() => {
      class DummyWebSocket extends EventTarget {
        constructor(url, protocols) {
          super();
          this.url = url;
          this.readyState = 0;
          this.bufferedAmount = 0;
          this.extensions = '';
          this.protocol = '';
          this.binaryType = 'blob';
        }
        send(data) {}
        close(code, reason) {
          this.readyState = 3;
        }
      }
      window.WebSocket = DummyWebSocket;
    });

    let foundTicket = null;
    page.on('console', msg => console.log('PAGE_LOG:', msg.text()));
    page.on('pageerror', err => console.log('PAGE_ERR:', err.message));
    page.on('request', req => {
      const url = req.url();
      if (url.includes('/auth/')) {
        console.log('AUTH_REQ:', req.method(), url);
      }
    });
    page.on('response', async (res) => {
      const url = res.url();
      if (url.includes('/auth/')) {
        console.log('AUTH_RES:', res.status(), url);
      }
      if (url.includes('/auth/world-ticket')) {
        try {
          const data = await res.json();
          console.log('TICKET_DATA:', JSON.stringify(data));
          if (data && data.ticket) {
            foundTicket = data.ticket;
          }
        } catch(e) {
          console.log('JSON_ERR:', e.message);
        }
      }
    });

    console.log('Navigating to play.tibidle.com...');
    await page.goto('https://play.tibidle.com/', { timeout: 30000, waitUntil: 'domcontentloaded' }).catch(e => console.log('NAV_ERR:', e.message));

    // Poll and click "ENTER THE GAME" / "ENTRAR NO JOGO" button
    console.log('Waiting for lobby and clicking enter...');
    let hasClicked = false;
    let clickTime = 0;
    const start = Date.now();
    while (Date.now() - start < 60000) {
      if (foundTicket) break;

      const btnInfo = await page.evaluate(() => {
        const btns = Array.from(document.querySelectorAll('button'));
        return btns.map(b => ({
          text: (b.innerText || '').trim(),
          disabled: b.disabled,
          classes: b.className
        }));
      }).catch(() => []);

      const enterBtn = btnInfo.find(b => {
        const t = b.text.toUpperCase();
        return t.includes('ENTER') || t.includes('ENTRAR') || t.includes('JOGO');
      });

      if (enterBtn) {
        console.log('FOUND_BTN:', JSON.stringify(enterBtn));
        if (!enterBtn.disabled && !hasClicked) {
          hasClicked = true;
          clickTime = Date.now();
          await page.evaluate(() => {
            const btns = Array.from(document.querySelectorAll('button'));
            const b = btns.find(btn => {
              const t = (btn.innerText || '').toUpperCase();
              return t.includes('ENTER') || t.includes('ENTRAR') || t.includes('JOGO');
            });
            if (b) b.click();
          });
          console.log('CLICKED_ACTIVE_ENTER_BTN!');
        }
      }

      if (hasClicked && Date.now() - clickTime > 30000) {
        console.log('Timeout waiting for ticket after click');
        break;
      }

      await new Promise(r => setTimeout(r, 1000));
    }

    await browser.close();

    if (foundTicket) {
      console.log('TICKET_FOUND:' + foundTicket);
      process.exit(0);
    } else {
      console.error('TICKET_NOT_FOUND');
      process.exit(1);
    }
  } catch (err) {
    try { await browser.close(); } catch(e) {}
    console.error('ERROR:' + err.message);
    process.exit(1);
  }
}

const cookieArg = process.argv[2] || 'did=14d2df158e5f0f5e3df5bdb755bec55100eee1ba2937fc585a65e5c57e8a4a65; sid=9189720e9b70b76e8f2cbf07fdc05ed127b343f7313d13619bf65b48a42358b7';
getTicket(cookieArg);
