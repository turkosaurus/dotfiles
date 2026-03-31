#!/usr/bin/env bash

# Universal dotfiles bootstrap — works on bare metal, containers, and VMs.
# Can be sourced (sources shell config after) or executed (prints restart message).

dot_dir="$HOME/dotfiles"
repo="https://github.com/turkosaurus/dotfiles"

red='\033[0;31m'
orange='\033[0;33m'
reset='\033[0m'

err() {
	echo -e "${red}ERR${reset}: $*" >&2
	return 1 2>/dev/null || exit 1
}

warn() {
	echo -e "${orange}WRN${reset}: $*" >&2
}

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
			if ! apt-get update -qq; then
				err "apt-get update failed"
			fi
			if ! apt-get install -y -qq "${missing[@]}"; then
				err "apt-get install failed"
			fi
		elif command -v sudo &>/dev/null; then
			if ! sudo apt-get update -qq; then
				err "apt-get update failed"
			fi
			if ! sudo apt-get install -y -qq "${missing[@]}"; then
				err "apt-get install failed"
			fi
		else
			warn "need root or sudo to install packages"
		fi
	fi
fi

# 2. Clone dotfiles
if [[ ! -d "$dot_dir/.git" ]]; then
	echo "cloning dotfiles..."
	if ! GIT_TERMINAL_PROMPT=0 git clone "$repo" "$dot_dir"; then
		err "failed to clone dotfiles"
	fi
fi

# 3. Oh-my-zsh (before dotsync so our .zshrc wins)
if [[ ! -d "$HOME/.oh-my-zsh" ]]; then
	echo "installing oh-my-zsh..."
	install_script="$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"
	if [[ -z "$install_script" ]]; then
		err "failed to download oh-my-zsh installer"
	fi
	if ! RUNZSH=no CHSH=no sh -c "$install_script" "" --unattended; then
		err "failed to install oh-my-zsh"
	fi
fi

# 4. Dotsync (local only — init.sh handles cloning/pulling)
echo "running dotsync..."
if ! "$dot_dir/home/bin/dotsync" -l; then
	err "dotsync failed"
fi

# 5. Mise
export PATH="$HOME/.local/bin:$HOME/bin:$PATH"
if ! command -v mise &>/dev/null; then
	echo "installing mise..."
	if ! "$dot_dir/home/bin/install/mise"; then
		err "failed to install mise"
	fi
fi

# 6. Install tools
echo "installing tools via mise..."
if ! cd "$HOME"; then
	err "cd $HOME failed"
fi
if ! mise trust; then
	err "mise trust failed"
fi
if ! MISE_PYTHON_PRECOMPILED_FLAVOR=install_only_stripped mise install; then
	err "mise install failed"
fi

# 7. Change shell to zsh
if [[ "$(basename "${SHELL:-}")" != "zsh" ]]; then
	zsh="$(command -v zsh || true)"
	if [[ -n "$zsh" ]]; then
		echo "changing shell to zsh..."
		if (( $(id -u) == 0 )); then
			chsh -s "$zsh" 2>/dev/null || true
		else
			sudo chsh -s "$zsh" "$(whoami)" 2>/dev/null ||
				chsh -s "$zsh" 2>/dev/null ||
				warn "could not change shell to zsh"
		fi
	fi
fi

echo "init complete."

# shellcheck disable=SC1090,SC1091
if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
	echo "sourcing shell config..."
	source "$HOME/.zshrc" 2>/dev/null ||
		source "$HOME/.bashrc" 2>/dev/null ||
		true
else
	echo "restart your shell or run: source ${BASH_SOURCE[0]}"
fi
