#!/usr/bin/env bash
# ==============================================================================
# Script de Instalação Automática do Tibidle Bot na VPS (Ubuntu/Debian)
# Otimizado para instâncias 1 vCPU / 1 GB RAM (Oracle Free Tier, AWS t2/t3.micro)
# ==============================================================================

set -e

echo "================================================================================"
echo "  ⚔  CONFIGURANDO TIBIDLE HEADLESS BOT (1 vCPU / 1 GB RAM)  ⚔"
echo "================================================================================"

# 1. Configurar SWAP de 2GB (evita OOM em VPS com 1GB de RAM)
if [ ! -f /swapfile ]; then
    echo "[1/5] Criando arquivo SWAP de 2GB..."
    fallocate -l 2G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=2048
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    echo "✓ SWAP ativado com sucesso."
else
    echo "[1/5] SWAP já existe. Pulando etapa."
fi

# 2. Atualizar repositórios e instalar dependências essenciais
echo "[2/5] Instalando dependências básicas e Chromium..."
apt-get update -y
apt-get install -y \
    curl \
    git \
    wget \
    golang-go \
    chromium-browser \
    ca-certificates

# 3. Compilar o bot nativo em Go
echo "[3/5] Compilando binário nativo em Go..."
go mod tidy
go build -ldflags="-s -w" -o tibidle-bot main.go
chmod +x tibidle-bot
echo "✓ Binário tibidle-bot compilado com sucesso!"

# 4. Configurar serviço Systemd para rodar 24/7 em background
echo "[4/5] Configurando serviço systemd..."
DIR=$(pwd)
cat <<EOF > /etc/systemd/system/tibidle.service
[Unit]
Description=Tibidle Native Go Bot Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=$DIR
ExecStart=$DIR/tibidle-bot
Restart=always
RestartSec=5
LimitNOFILE=65535
Environment=PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable tibidle.service

echo "[5/5] Concluído!"
echo "================================================================================"
echo "  ✓ Instalação finalizada com sucesso!"
echo "  "
echo "  Comandos úteis:"
echo "    - Iniciar no terminal agora:   ./tibidle-bot"
echo "    - Iniciar como serviço 24/7:   systemctl start tibidle"
echo "    - Ver status do serviço:       systemctl status tibidle"
echo "    - Ver logs em tempo real:      journalctl -u tibidle -f"
echo "    - Dashboard Web:               http://SEU_IP_VPS:3000"
echo "================================================================================"
