#!/usr/bin/env bash

set -e

# Git push to SSH remote script
# Usage: ./git-push-ssh.sh [remote-name] [branch]

if [ -z "$1" ]; then
	echo "Usage: $0 [remote-name] [branch]"
	exit 1
fi
REMOTE="$1"
BRANCH="${2:-$(git branch --show-current)}"

# Check if we're in a git repo
if ! git rev-parse --git-dir >/dev/null 2>&1; then
	echo "Error: Not in a git repository"
	exit 1
fi

# Check if remote exists
if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
	echo "Error: Remote '$REMOTE' does not exist"
	echo ""
	echo "Add it with:"
	echo "  git remote add $REMOTE user@hostname:/path/to/repo"
	exit 1
fi

# Warn about uncommitted changes
if ! git diff-index --quiet HEAD --; then
	echo "Warning: You have uncommitted changes"
	echo "These will NOT be pushed. Commit them first if needed."
	echo ""
fi

# Show what we're doing
REMOTE_URL=$(git remote get-url "$REMOTE")
echo "Pushing branch '$BRANCH' to '$REMOTE' ($REMOTE_URL)"
echo ""

# Push
git push "$REMOTE" "$BRANCH"

echo ""
echo "✓ Successfully pushed to $REMOTE"
