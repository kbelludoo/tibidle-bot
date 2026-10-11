#!/bin/bash
STATUS=$(curl -s http://127.0.0.1:3001/api/status)
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
    -f message="telemetry: live bot status update" \
    -f content="$B64" \
    -f sha="$SHA" > /dev/null 2>&1
else
  gh api -X PUT repos/kbelludoo/tibidle-bot/contents/docs/status.json \
    -f message="telemetry: live bot status update" \
    -f content="$B64" > /dev/null 2>&1
fi
echo "Telemetry synced to GitHub Pages!"
