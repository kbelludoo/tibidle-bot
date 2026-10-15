#!/bin/bash
# Sincroniza a telemetria do Tibidle para o GitHub Pages (docs/status.json)
# MODO STEALTH: name, id, e-mails e IPs são anonimizados com jq ANTES do push.
STATUS=$(curl -s http://127.0.0.1:3001/api/status)
if [ -z "$STATUS" ] || [ "$STATUS" = "null" ]; then
  exit 0
fi

STATUS=$(echo "$STATUS" | jq '
  (if (.accounts | type) == "array" then
    .accounts |= (to_entries | map(
        .key as $i
        | .value
        | .id = ("acc" + (($i + 1) | tostring))
        | .name = ("Conta #" + (($i + 1) | tostring))
        | if (.recentLogs | type) == "array" then
            .recentLogs |= map(
              gsub("Conectado como [^)]*\\)"; ("Conectado como Conta #" + (($i + 1) | tostring)))
            )
          else . end
      ))
   else . end)
  | walk(
      if type == "string" and (test("^https?://") | not) then
        gsub("[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"; "usuario@privado.com")
        | gsub("\\b[0-9]{1,3}(\\.[0-9]{1,3}){3}\\b"; "x.x.x.x")
        | gsub("(?i)kbelludoo2|kbelludoo|lohmoreira"; "treinador")
      else . end
    )
')

if [ -z "$STATUS" ] || [ "$STATUS" = "null" ]; then
  exit 0
fi

TUNNEL_URL=$(grep -o 'https://[a-zA-Z0-9.-]*\.trycloudflare\.com' /home/ubuntu/tibidle-tunnel.log 2>/dev/null | tail -n 1)
if [ -n "$TUNNEL_URL" ]; then
  STATUS=$(echo "$STATUS" | jq --arg u "$TUNNEL_URL" '. + {tunnelUrl: $u}')
fi

B64=$(echo "$STATUS" | base64 -w 0)
SHA=$(gh api repos/kbelludoo/tibidle-bot/contents/docs/status.json --jq .sha 2>/dev/null)

if [ -n "$SHA" ]; then
  gh api -X PUT repos/kbelludoo/tibidle-bot/contents/docs/status.json \
    -f message="telemetry: live bot status update (stealth)" \
    -f content="$B64" \
    -f sha="$SHA" > /dev/null 2>&1
else
  gh api -X PUT repos/kbelludoo/tibidle-bot/contents/docs/status.json \
    -f message="telemetry: live bot status update (stealth)" \
    -f content="$B64" > /dev/null 2>&1
fi
echo "Telemetry synced to GitHub Pages (anonymous mode)!"
