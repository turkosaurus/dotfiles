#!/usr/bin/env bash
# List Slack messages I've reacted to with :eyes: but not :white_check_mark:.
# Usage: slack-todo.sh [DAYS]   (default 30)

set -euo pipefail

days="${1:-30}"
cutoff=$(date -v-"${days}"d +%s 2>/dev/null || date -d "${days} days ago" +%s)

: "${SLACK_USER_TOKEN:?SLACK_USER_TOKEN not set}"
me="${SLACK_USER_ID:-U03RB04NY8P}"
open="${SLACK_TODO_OPEN:-eyes}"
done_r="${SLACK_TODO_DONE:-white_check_mark}"

cache_dir="${XDG_CACHE_HOME:-$HOME/.cache}"
mkdir -p "$cache_dir"
users_cache="$cache_dir/slack-users.json"

slack_get() {
  local url="$1" cursor="${2:-}" hdr body status retry
  hdr=$(mktemp); body=$(mktemp)
  trap 'rm -f "$hdr" "$body"' RETURN
  local args=(
    -sS -G -D "$hdr" -o "$body" -w '%{http_code}'
    -H "Authorization: Bearer $SLACK_USER_TOKEN"
    --data-urlencode "limit=200"
  )
  [[ -n "$cursor" ]] && args+=(--data-urlencode "cursor=$cursor")
  args+=("$url")
  while :; do
    status=$(curl "${args[@]}")
    if [[ "$status" == "429" ]]; then
      retry=$(awk 'tolower($1)=="retry-after:"{print $2+0; exit}' "$hdr")
      sleep "${retry:-5}"
      continue
    fi
    break
  done
  cat "$body"
}

fetch_users() {
  local cursor="" all="[]" resp
  while :; do
    resp=$(slack_get "https://slack.com/api/users.list" "$cursor")
    if [[ "$(jq -r '.ok' <<<"$resp")" != "true" ]]; then
      echo "slack-todo: users.list: $(jq -r '.error' <<<"$resp")" >&2
      return 1
    fi
    all=$(jq -c --argjson new "$(jq '.members' <<<"$resp")" '. + $new' <<<"$all")
    cursor=$(jq -r '.response_metadata.next_cursor // ""' <<<"$resp")
    [[ -z "$cursor" ]] && break
  done
  jq -c 'map({key: .id, value: .name}) | from_entries' <<<"$all"
}

if [[ ! -s "$users_cache" ]] || [[ -z "$(find "$users_cache" -mtime -1 2>/dev/null)" ]]; then
  fetch_users > "$users_cache"
fi
users=$(cat "$users_cache")

base="${SLACK_WORKSPACE_URL:-}"
cursor=""
while :; do
  resp=$(slack_get "https://slack.com/api/reactions.list" "$cursor")

  if [[ "$(jq -r '.ok' <<<"$resp")" != "true" ]]; then
    echo "slack-todo: $(jq -r '.error // "unknown"' <<<"$resp")" >&2
    exit 1
  fi

  if [[ -z "$base" ]]; then
    pl=$(jq -r '[.items[] | .message.permalink // empty] | first // ""' <<<"$resp")
    [[ -n "$pl" ]] && base="${pl%/archives/*}"
  fi

  jq -c --arg base "$base" \
        --argjson users "$users" \
        --argjson cutoff "$cutoff" \
        --arg me "$me" --arg open "$open" --arg done "$done_r" '
    .items[]
    | select(.type == "message" and .message.reactions != null)
    | select((.message.ts | tonumber) >= $cutoff)
    | select(any(.message.reactions[];
        .name == $open and (.users | index($me))))
    | select((any(.message.reactions[];
        .name == $done and (.users | index($me)))) | not)
    | {
        ts: .message.ts,
        channel,
        author: ($users[.message.user] // .message.user),
        text: ((.message.text // "") | .[0:200]),
        permalink: (.message.permalink //
          (if $base != "" then
             "\($base)/archives/\(.channel)/p\(.message.ts | gsub("\\."; ""))"
           else null end))
      }
  ' <<<"$resp"

  past_cutoff=$(jq -r --argjson cutoff "$cutoff" '
    ([.items[].message.ts // empty | tonumber] | min // ($cutoff + 1)) < $cutoff
  ' <<<"$resp")
  [[ "$past_cutoff" == "true" ]] && break

  cursor=$(jq -r '.response_metadata.next_cursor // ""' <<<"$resp")
  [[ -z "$cursor" ]] && break
done
