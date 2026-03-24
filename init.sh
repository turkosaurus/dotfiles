#!/usr/bin/env bash

# Universal dotfiles bootstrap — works on bare metal, containers, and VMs.
# Can be sourced (sources shell config after) or executed (prints restart message).

set -eo pipefail

dot_dir="$HOME/dotfiles"
repo="https://github.com/turkosaurus/dotfiles"

# 1. Install system deps (Linux only)
if [[ "$(uname -s)" == "Linux" ]]; then
	pkgs=(git curl zsh sudo build-essential)
	missing=()
	for p in "${pkgs[@]}"; do
		dpkg -s "$p" &>/dev/null || missing+=("$p")
	done
	if (( ${#missing[@]} )); then
		echo "installing: ${missing[*]}"
		if (( $(id -u) == 0 )); then
			apt-get update -qq && apt-get install -y -qq "${missing[@]}"
		elif command -v sudo &>/dev/null; then
			sudo apt-get update -qq && sudo apt-get install -y -qq "${missing[@]}"
		else
			echo "warning: need root or sudo to install packages" >&2
		fi
	fi
fi

# 2. Clone dotfiles
if [[ ! -d "$dot_dir/.git" ]]; then
	echo "cloning dotfiles..."
	GIT_TERMINAL_PROMPT=0 git clone "$repo" "$dot_dir"
fi

# 3. Oh-my-zsh (before dotsync so our .zshrc wins)
if [[ ! -d "$HOME/.oh-my-zsh" ]]; then
	echo "installing oh-my-zsh..."
	RUNZSH=no CHSH=no sh -c \
		"$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)" \
		"" --unattended
fi

# 4. Dotsync (local only — init.sh handles cloning/pulling)
echo "running dotsync..."
"$dot_dir/home/bin/dotsync" -l

# 5. Mise
export PATH="$HOME/.local/bin:$PATH"
if ! command -v mise &>/dev/null; then
	echo "installing mise..."
	"$dot_dir/home/bin/install/mise"
fi

# 6. Install tools
echo "installing tools via mise..."
cd "$HOME"
mise trust
MISE_PYTHON_PRECOMPILED_FLAVOR=install_only_stripped mise install

# 7. Change shell to zsh
if [[ "$(basename "${SHELL:-}")" != "zsh" ]]; then
	zsh="$(command -v zsh || true)"
	if [[ -n "$zsh" ]]; then
		echo "changing shell to zsh..."
		if (( $(id -u) == 0 )); then
			chsh -s "$zsh" 2>/dev/null || true
		else
			sudo chsh -s "$zsh" "$(whoami)" 2>/dev/null || \
				chsh -s "$zsh" 2>/dev/null || \
				echo "warning: could not change shell to zsh" >&2
		fi
	fi
fi

echo "init complete."

# shellcheck disable=SC1090,SC1091
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
	echo "sourcing shell config..."
	source "$HOME/.zshrc" 2>/dev/null || \
		source "$HOME/.bashrc" 2>/dev/null || true
else
	echo "restart your shell or run: source ${BASH_SOURCE[0]}"
fi
