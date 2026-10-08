// ==============================================================================
// Tibidle Session Exporter — Cole no Console (F12) em https://play.tibidle.com/
// ==============================================================================
(function() {
  const sessionData = {
    cookies: document.cookie,
    sessionProof: sessionStorage.getItem("tibidle.sessionProof") || "",
    localStorage: Object.keys(localStorage).reduce((acc, k) => {
      acc[k] = localStorage.getItem(k);
      return acc;
    }, {})
  };

  const jsonStr = JSON.stringify(sessionData, null, 2);
  copy(jsonStr);

  console.log("%c[Tibidle Bot] Sessão copiada com sucesso para sua área de transferência!", "color: #10b981; font-weight: bold; font-size: 14px;");
  console.log("Cole o conteúdo no arquivo session.json ou no campo sessionCookie de accounts.json na sua VPS.");
})();
