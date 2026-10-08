# ⚔ Tibidle Multi-Bot Nativo em Go (Headless / Ultra-Leve)

Bot de alta performance desenvolvido em **Go nativo** para rodar o jogo [Tibidle](https://play.tibidle.com/) em VPS de baixo custo (**1 vCPU / 1 GB RAM**) com consumo recorde de memória (**~1 MB a 3 MB de RAM**), sem qualquer navegador instalado na VPS e com automação completa.

Inspirado na arquitetura do [huntera-bot](https://github.com/kbelludoo/huntera-bot).

🌐 **GitHub Pages Live Dashboard:** [https://kbelludoo.github.io/tibidle-bot/](https://kbelludoo.github.io/tibidle-bot/)

---

## ⚡ Comparativo de Consumo

| Recurso | Navegador Comum / Puppeteer | Tibidle Bot em Go (Headless) |
| :--- | :--- | :--- |
| **Uso de RAM** | 800 MB ~ 1.5 GB *(Dá crash/OOM na VPS)* | **~1.0 MB a 3.0 MB** *(Zero crash, ultra-leve)* |
| **Uso de CPU** | 40% ~ 90% *(Renderizando sprites/WebGL)* | **0.1% ~ 0.5%** *(Apenas troca de pacotes)* |
| **Navegador na VPS** | Exige Chrome + Xvfb pesados | **ZERO navegadores na VPS** *(100% Go nativo)* |
| **Tráfego de Rede** | Download constante de texturas/áudio | **Apenas WebSockets essenciais** |
| **Dashboard** | Janela pesada de navegador | **Web Server embutido (Porta 3001) + GitHub Pages** |

---

## 🤖 Funcionalidades Automáticas (Full Auto)

- **Auto-Hunt contínuo:** Inicia e reinicia caçadas automaticamente (com lure customizado, seleção de hunt por nome ou ID).
- **Auto-Sell:** Vende automaticamente todo o loot de criaturas no NPC para gerar ouro puro continuamente.
- **Auto-Blessings:** Compra todas as proteções/bênçãos periodicamente.
- **Auto-Daily Quests:** Coleta missões e recompensas diárias do servidor.
- **Auto-Potions & Spells:** Usa magias em área e poções quando o HP ou Mana baixam.
- **Anti-Idle / Keepalive:** Mantém a conexão WebSocket viva 24/7 sem ser desconectado por inatividade.
- **Multi-Account:** Roda múltiplas contas simultâneas em Goroutines nativas isoladas.
- **Web Dashboard Embutido (Porta 3001) & GitHub Pages:** Painel web dark-mode em tempo real para acompanhar HP, Mana, Exp/h, Ouro, Abates e uso de RAM pelo celular ou navegador.

---

## 🚀 Instalação e Execução na VPS

### 1. Na VPS (Ubuntu / Debian):
Clone o repositório ou suba os arquivos para a VPS:
```bash
git clone https://github.com/kbelludoo/tibidle-bot.git ~/tibidle-bot
cd ~/tibidle-bot
```

### 2. Configurar o Serviço Systemd 24/7:
```bash
sudo cp tibidle.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable tibidle
sudo systemctl start tibidle
```

Para verificar os logs em tempo real na VPS:
```bash
sudo journalctl -u tibidle -f
```

---

## 🎫 Como Conectar / Enviar Ticket de Jogo

O jogo utiliza proteção Cloudflare Turnstile no endpoint de handshake inicial. Como a VPS não possui navegadores instalados, você pode gerar o ticket no seu computador em **3 segundos** e ele é entregue automaticamente para a VPS:

```powershell
go run ./cmd/get_ticket
```

O comando:
1. Abre rapidamente o navegador local por 3 segundos.
2. Captura o ticket oficial do jogo.
3. Envia com segurança via SSH/API diretamente para o bot na VPS (`http://127.0.0.1:3001/api/ticket`).
4. O bot conecta e continua caçando continuamente 24/7!

---

## 🌐 Dashboard Web e GitHub Pages

- **GitHub Pages:** [https://kbelludoo.github.io/tibidle-bot/](https://kbelludoo.github.io/tibidle-bot/)
- **Dashboard Direto na VPS:** `http://IP_DA_SUA_VPS:3001/`
