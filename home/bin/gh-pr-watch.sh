#!/usr/bin/env bash

# Watch for copilot review on current branch's PR, then launch /pr-review.
#
# Usage: gh-watch.sh [--now]
#   --now  Act on existing copilot review immediately

set -euo pipefail

NOW=false
if [[ "${1:-}" == "--now" ]]; then
    NOW=true
fi

pr=$(gh pr view --json number -q .number) || {
    echo "error: no PR found for current branch" >&2
    exit 1
}
repo=$(gh repo view --json nameWithOwner -q .nameWithOwner)
last_id=""
initialized=false

# Re-trigger copilot review by resolving its old threads (unless already pending)
pending=$(gh api "repos/${repo}/pulls/${pr}/requested_reviewers" \
    --jq '[.users[]? | select(.login == "Copilot")] | length' 2>/dev/null) || pending=0

if [[ "$pending" -gt 0 ]]; then
    echo "copilot review already pending on PR #${pr}"
else
    owner="${repo%%/*}"
    name="${repo##*/}"
    thread_ids=$(gh api graphql -f query='
      query($owner: String!, $name: String!, $pr: Int!) {
        repository(owner: $owner, name: $name) {
          pullRequest(number: $pr) {
            reviewThreads(first: 100) {
              nodes {
                id
                isResolved
                comments(first: 1) {
                  nodes { author { login } }
                }
              }
            }
          }
        }
      }' -f owner="$owner" -f name="$name" -F pr="$pr" \
        --jq '.data.repository.pullRequest.reviewThreads.nodes[]
            | select(.isResolved == false)
            | select(.comments.nodes[0].author.login == "copilot-pull-request-reviewer[bot]")
            | .id' 2>/dev/null) || thread_ids=""

    if [[ -n "$thread_ids" ]]; then
        echo "resolving copilot threads to trigger re-review..."
        while IFS= read -r tid; do
            gh api graphql -f query='
              mutation($id: ID!) {
                resolveReviewThread(input: {threadId: $id}) {
                  thread { isResolved }
                }
              }' -f id="$tid" >/dev/null 2>&1 || true
        done <<<"$thread_ids"
    else
        pr_url=$(gh pr view "$pr" --json url -q .url)
        echo "no unresolved copilot threads, rerequest to continue"
        echo "$pr_url"
    fi
fi

# Record baseline review id
last_id=$(gh api "repos/${repo}/pulls/${pr}/reviews" \
    --jq '[.[] | select(.user.login == "copilot-pull-request-reviewer[bot]")] | last | .id // empty' 2>/dev/null) || last_id=""

# Phase 1: wait for pending review (user may need to trigger manually)
if [[ "$pending" -eq 0 ]]; then
    printf "waiting for copilot review request on PR #${pr}..."
    while true; do
        pending=$(gh api "repos/${repo}/pulls/${pr}/requested_reviewers" \
            --jq '[.users[]? | select(.login == "Copilot")] | length' 2>/dev/null) || pending=0
        if [[ "$pending" -gt 0 ]]; then
            printf "\n"
            break
        fi
        printf .
        sleep 15
    done
fi

# Phase 2: wait for review to complete
printf "copilot review in progress on PR #${pr}..."
while true; do
    review_id=$(gh api "repos/${repo}/pulls/${pr}/reviews" \
        --jq '[.[] | select(.user.login == "copilot-pull-request-reviewer[bot]")] | last | .id // empty' 2>/dev/null) || review_id=""

    if [[ -n "$review_id" && "$review_id" != "$last_id" ]]; then
        printf "\ncopilot review detected, launching pr-review...\n"
        claude "/pr-review ${pr}"
        break
    fi

    printf .
    sleep 15
done
