#!/usr/bin/env bash
# check-agent-access.sh — verify the agent-facing .txt surface is reachable
# without bot challenges, across several unverified-bot and plain-client UAs.
# What this proves: unverified-bot UA + runner-IP traffic gets 200s with
# expected bodies and no Cloudflare challenge, right now, from this runner.
# What this does NOT prove: verified-bot (e.g. Cloudflare-verified crawler)
# handling, or behavior for IPs with poor reputation — runner IPs are clean.
# Exit 0 = pass, 1 = blocked/challenged, 2 = network error (no block seen).
set -u

BASE="${PYPX_BASE_URL:-https://pypx.app}"

URLS=(
  "/llms.txt"
  "/api/packages/requests/summary.txt"
  "/api/search.txt?q=http&limit=3"
)

UAS=(
  "curl/8.6.0"
  "Mozilla/5.0 (compatible; ClaudeBot/1.0; +claudebot@anthropic.com)"
  "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; GPTBot/1.1; +https://openai.com/gptbot"
  "python-requests/2.32.0"
  "claude-code/2.0"
)

# Extra header per UA, aligned by index. Empty string = no extra header.
UA_EXTRA_HEADERS=(
  ""
  ""
  ""
  ""
  "Accept: text/markdown"
)

fail=0
net=0
for i in "${!UAS[@]}"; do
  ua="${UAS[$i]}"
  extra_header="${UA_EXTRA_HEADERS[$i]}"
  for path in "${URLS[@]}"; do
    url="${BASE}${path}"
    headers=$(mktemp)
    curl_args=(-sS -D "$headers" -A "$ua" -m 20 --retry 3 --retry-connrefused --retry-delay 2)
    if [ -n "$extra_header" ]; then
      curl_args+=(-H "$extra_header")
    fi
    body=$(curl "${curl_args[@]}" "$url")
    rc=$?
    if [ $rc -ne 0 ]; then
      echo "NETWORK FAIL rc=$rc  $url  [UA: $ua]"
      rm -f "$headers"
      net=1
      continue
    fi
    status=$(grep -E '^HTTP/' "$headers" | tail -1 | awk '{print $2}')
    ctype=$(grep -i '^content-type:' "$headers" | tr -d '\r' | awk '{print $2}')
    mitigated=$(grep -ci '^cf-mitigated:' "$headers")

    body_ok=1
    case "$path" in
      /llms.txt)
        grep -qi 'pypx' <<<"$body" || body_ok=0
        ;;
      */summary.txt)
        grep -qi 'name: requests' <<<"$body" || body_ok=0
        ;;
      */search.txt*)
        grep -qE '^#' <<<"$body" || body_ok=0
        ;;
    esac

    if [ "$status" != "200" ] || [ "$mitigated" != "0" ] \
       || [[ "$ctype" != text/plain* ]] \
       || [ "$body_ok" != "1" ] \
       || grep -qiE "just a moment|attention required|challenge-platform" <<<"$body"; then
      echo "BLOCKED  status=$status ctype=$ctype cf-mitigated=$mitigated body_ok=$body_ok  $url  [UA: $ua]"
      fail=1
    else
      extra=""
      if [ "$path" = "/llms.txt" ]; then
        ccs=$(grep -i '^cf-cache-status:' "$headers" | tr -d '\r' | awk '{print $2}')
        extra="  cf-cache-status=${ccs:-none}"
      fi
      echo "OK       $url  [UA: $ua]${extra}"
    fi
    rm -f "$headers"
  done
done

if [ "$fail" != "0" ]; then
  exit 1
elif [ "$net" != "0" ]; then
  exit 2
else
  exit 0
fi
