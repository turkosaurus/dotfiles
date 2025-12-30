#!/usr/bin/env bash

set -euo pipefail

dot_path="$HOME/dotfiles"
repo_path="https://github.com/turkosaurus/dotfiles"
if [[ ! -d "$dot_path" ]]; then
    echo "dotfiles not found, cloning anew... ($dot_path)"
	if ! command -v git &> /dev/null; then
	    echo "git required but not found"
	    exit 1
	fi
	# suppress any login requirements 
	if ! GIT_TERMINAL_PROMPT=0 git clone "$repo_path" "$dot_path"; then
	    echo "error: git clone failed"
	    exit 1
	fi
fi

echo "running dotsync..."
cd "$dot_path" || exit 1
./home/bin/dotsync -v

dot_bin_path="$dot_path/bin"
echo "updating path: PATH=\$PATH:$dot_bin_path"
export PATH="$PATH:$dot_bin_path"

# shellcheck disable=SC1090
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
    echo "sourcing shell config..."
    # Source the shell config we may have just created, modified, or linked.
    case "$SHELL" in
	*bash)
	    echo "Sourcing bashrc..."
	    source ~/.bashrc
	    ;;
	*zsh)
	    echo "Sourcing zshrc..."
	    source ~/.zshrc
	    ;;
	*)
	    echo "No config file to source for shell: $SHELL"
	    ;;
    esac
else
    echo "cannot update shell config, not sourced."
    echo "restart your terminal to apply changes, or run:"
    echo "  source $0"
fi

echo "dotfiles init complete."

