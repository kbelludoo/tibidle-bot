@echo off
chcp 65001 >nul
title Tibidle Bot - Conectar e Enviar Ticket
cd /d "%~dp0"

echo ================================================================================
echo   ⚔  TIBIDLE BOT - CONECTAR COM 1 CLIQUE  ⚔
echo ================================================================================
echo.
echo [1/2] Abrindo navegador para autenticar e gerar o ticket do jogo...
echo (O navegador vai abrir. Se aparecer "Confirme que voce nao e um robo", basta marcar a caixinha).
echo.

go run ./cmd/get_ticket

echo.
echo ================================================================================
echo   Verificando status do bot na VPS...
echo ================================================================================
echo.
pause
