#!/usr/bin/env bash
# Probes each smoke-test provider key with one minimal call. On an auth, billing,
# or missing-secret failure, it opens or updates one GitHub issue and exits 1.
# Transient errors (rate limit, network, 5xx) print a row but do not fail the run.
set -euo pipefail

readonly ISSUE_TITLE="Provider API key check failed"
readonly DETAIL_MAX=200

# classify prints ok, auth, billing, rate_limit, or error for an HTTP status and body.
# Billing body patterns run before status codes because OpenAI sends insufficient_quota with HTTP 429.
classify() {
  local code=$1 body=$2
  if [[ $code == 2?? ]]; then
    echo ok
    return
  fi
  if grep -Eqi 'insufficient_quota|credit_balance|credit balance|billing_error|insufficient credits|spend_limit_exceeded|usage_limit_exceeded' <<<"$body"; then
    echo billing
    return
  fi
  case $code in
    401 | 403) echo auth ;;
    402 | 432 | 433) echo billing ;;
    429) echo rate_limit ;;
    *) echo error ;;
  esac
}

# probe sends one request for a provider and prints a TSV row: name, class, HTTP code, detail.
# Args: name, key, url, then extra curl args. The key never appears in the detail.
probe() {
  local name=$1 key=$2 url=$3
  shift 3
  if [[ -z $key ]]; then
    printf '%s\tmissing\t000\tsecret is not set\n' "$name"
    return
  fi
  local body_file code body cls detail
  body_file=$(mktemp)
  code=$(curl -sS --max-time 60 -o "$body_file" -w '%{http_code}' "$@" "$url" 2>/dev/null) || code=000
  body=$(<"$body_file")
  rm -f "$body_file"
  cls=$(classify "$code" "$body")
  detail=${body//"$key"/[REDACTED]}
  detail=${detail//$'\n'/ }
  detail=${detail:0:$DETAIL_MAX}
  [[ $cls == ok ]] && detail=""
  printf '%s\t%s\t%s\t%s\n' "$name" "$cls" "$code" "$detail"
}

# probe_providers runs one minimal call per provider. Each call uses the cheapest
# settings: one output token, or one search for Tavily.
probe_providers() {
  local anthropic=${ANTHROPIC_API_KEY:-} openai=${OPENAI_API_KEY:-}
  local openrouter=${OPENROUTER_API_KEY:-} tavily=${TAVILY_API_KEY:-}
  probe anthropic "$anthropic" "${AXE_ANTHROPIC_BASE_URL:-https://api.anthropic.com}/v1/messages" \
    -H "x-api-key: $anthropic" -H "anthropic-version: 2023-06-01" -H "content-type: application/json" \
    -d '{"model":"claude-haiku-4-5","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}'
  probe openai "$openai" "${AXE_OPENAI_BASE_URL:-https://api.openai.com/v1}/chat/completions" \
    -H "Authorization: Bearer $openai" -H "content-type: application/json" \
    -d '{"model":"gpt-4o-mini","max_completion_tokens":1,"messages":[{"role":"user","content":"ping"}]}'
  probe openrouter "$openrouter" "${AXE_OPENROUTER_BASE_URL:-https://openrouter.ai/api/v1}/chat/completions" \
    -H "Authorization: Bearer $openrouter" -H "content-type: application/json" \
    -d '{"model":"minimax/minimax-m2.5","max_tokens":1,"messages":[{"role":"user","content":"ping"}]}'
  probe tavily "$tavily" "${AXE_TAVILY_BASE_URL:-https://api.tavily.com}/search" \
    -H "Authorization: Bearer $tavily" -H "content-type: application/json" \
    -d "$(jq -cn --arg k "$tavily" '{query:"ping",max_results:1,search_depth:"basic",api_key:$k}')"
}

# upsert_issue opens the failure issue, or edits the open one, so only one issue exists.
# Args: formatted report rows.
upsert_issue() {
  local body_file number run_url=""
  body_file=$(mktemp)
  if [[ -n ${GITHUB_RUN_ID:-} ]]; then
    run_url="$GITHUB_SERVER_URL/$GITHUB_REPOSITORY/actions/runs/$GITHUB_RUN_ID"
  fi
  {
    echo "A provider API key failed the scheduled check. Rotate or fund each key listed below, or restore a missing secret, then close this issue."
    echo
    printf '%s\n' "$@"
    if [[ -n $run_url ]]; then
      echo
      echo "Run: $run_url"
    fi
  } >"$body_file"
  number=$(gh issue list --state open --limit 200 --json number,title \
    --jq "[.[] | select(.title == \"$ISSUE_TITLE\") | .number] | first // empty")
  if [[ -n $number ]]; then
    gh issue edit "$number" --body-file "$body_file"
  else
    gh issue create --title "$ISSUE_TITLE" --body-file "$body_file"
  fi
  rm -f "$body_file"
}

main() {
  local -a rows=()
  local name cls code detail failed=0
  while IFS=$'\t' read -r name cls code detail; do
    rows+=("$(printf '%-11s %-10s HTTP %s %s' "$name" "$cls" "$code" "$detail")")
    case $cls in
      auth | billing | missing) failed=1 ;;
    esac
  done < <(probe_providers)
  printf '%s\n' "${rows[@]}"
  if ((failed)); then
    upsert_issue "${rows[@]}"
  fi
  exit "$failed"
}

if [[ ${BASH_SOURCE[0]} == "$0" ]]; then
  main "$@"
fi
