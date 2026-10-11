// ==UserScript==
// @name         Tibidle Bot — Auto-Renew 1-Clique
// @namespace    https://kbelludoo.github.io/tibidle-bot/
// @version      1.0
// @description  Renova o ticket do bot na VPS automaticamente ao abrir o Tibidle
// @author       kbelludoo
// @match        https://play.tibidle.com/*
// @grant        none
// @run-at       document-idle
// ==/UserScript==

(function() {
  'use strict';

  // Cria botão flutuante discreto no canto da tela do jogo
  const btn = document.createElement('button');
  btn.innerText = '⚡ Renovar Bot VPS';
  btn.style.cssText = `
    position: fixed;
    bottom: 15px;
    right: 15px;
    z-index: 999999;
    background: #f59e0b;
    color: #000;
    font-weight: bold;
    font-size: 13px;
    padding: 10px 16px;
    border: 2px solid #fff;
    border-radius: 50px;
    box-shadow: 0 4px 15px rgba(0,0,0,0.5);
    cursor: pointer;
    font-family: sans-serif;
  `;

  btn.onclick = async () => {
    btn.innerText = '⏳ Conectando...';
    btn.style.background = '#3b82f6';
    btn.style.color = '#fff';

    try {
      // 1. Obter status e URL do tunnel via GitHub Pages
      const ghRes = await fetch('https://raw.githubusercontent.com/kbelludoo/tibidle-bot/main/docs/status.json?t=' + Date.now());
      const ghData = await ghRes.json();
      const targetUrl = ghData.tunnelUrl || 'https://rough-jennifer-tried-green.trycloudflare.com';

      // 2. Obter nonce
      const nonceRes = await (await fetch('/auth/world-nonce', { method: 'POST' })).json();
      if (!nonceRes || !nonceRes.nonce) throw new Error('Não foi possível obter o nonce.');

      // 3. Obter ticket
      const ticketRes = await (await fetch('/auth/world-ticket', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          laudo: { v: 1, nonce: nonceRes.nonce, versao: 'detector-2', t: Date.now(), sinais: {} }
        })
      })).json();

      if (!ticketRes || !ticketRes.ticket) throw new Error('Falha ao gerar ticket. Verifique se o jogo está logado.');

      // 4. Enviar para a VPS
      const postRes = await fetch(targetUrl.replace(/\/$/, '') + '/api/ticket', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ticket: ticketRes.ticket })
      });
      const postData = await postRes.json();

      if (postRes.ok && postData.ok) {
        btn.innerText = '✔ Bot Conectado!';
        btn.style.background = '#10b981';
        setTimeout(() => {
          btn.innerText = '⚡ Renovar Bot VPS';
          btn.style.background = '#f59e0b';
          btn.style.color = '#000';
        }, 3000);
      } else {
        throw new Error(postData.error || 'Erro ao entregar ticket na VPS.');
      }
    } catch (err) {
      alert('Erro ao renovar bot: ' + err.message);
      btn.innerText = '❌ Falha';
      btn.style.background = '#ef4444';
      setTimeout(() => {
        btn.innerText = '⚡ Renovar Bot VPS';
        btn.style.background = '#f59e0b';
        btn.style.color = '#000';
      }, 3000);
    }
  };

  document.body.appendChild(btn);
})();
