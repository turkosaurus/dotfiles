#!/usr/bin/env bash

# syncs all dotfiles
set -euo pipefail

echo "running dotsync..."
./home/bin/dotsync -v

dot_bin_path="$HOME/dotfiles/bin"
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

