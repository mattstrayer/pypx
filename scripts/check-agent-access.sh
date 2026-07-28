#!/usr/bin/env bash
# check-agent-access.sh — verify the agent-facing .txt surface is reachable
# without bot challenges. Exit 0 = pass, 1 = blocked/challenged, 2 = network error.
set -u

BASE="${PYPX_BASE_URL:-https://pypx.app}"

URLS=(
  "/llms.txt"
  "/api/packages/requests/summary.txt"
  "/api/search.txt?q=http&limit=3"
)

UAS=(
  "curl/8.6.0"
  "ClaudeBot/1.0 (+claudebot@anthropic.com)"
  "GPTBot/1.1 (+https://openai.com/gptbot)"
  "python-requests/2.32.0"
)

fail=0
for ua in "${UAS[@]}"; do
  for path in "${URLS[@]}"; do
    url="${BASE}${path}"
    headers=$(mktemp)
    body=$(curl -sS -D "$headers" -A "$ua" -m 20 "$url" 2>/dev/null)
    rc=$?
    if [ $rc -ne 0 ]; then
      echo "NETWORK FAIL rc=$rc  $url  [UA: $ua]"
      rm -f "$headers"
      exit 2
    fi
    status=$(head -1 "$headers" | awk '{print $2}')
    ctype=$(grep -i '^content-type:' "$headers" | tr -d '\r' | awk '{print $2}')
    mitigated=$(grep -ci '^cf-mitigated:' "$headers")
    rm -f "$headers"

    if [ "$status" != "200" ] || [ "$mitigated" != "0" ] \
       || [[ "$ctype" != text/plain* ]] \
       || grep -qiE "just a moment|attention required|challenge-platform" <<<"$body"; then
      echo "BLOCKED  status=$status ctype=$ctype cf-mitigated=$mitigated  $url  [UA: $ua]"
      fail=1
    else
      echo "OK       $url  [UA: $ua]"
    fi
  done
done

exit $fail
