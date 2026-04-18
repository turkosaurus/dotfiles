# --- oh-my-zsh ---
export ZSH="$HOME/.oh-my-zsh"
export ZSH_THEME="agnoster"
export AGNOSTER_CONTEXT_BG=magenta
export AGNOSTER_CONTEXT_FG=black
plugins=(git mise)
#shellcheck disable=SC1091
source "$ZSH/oh-my-zsh.sh"

export PATH=$PATH:~/.local/bin:~/bin
export EDITOR=nvim

# --- shared aliases ---
#
[[ -f ~/.aliases ]] && source ~/.aliases

# --- fzf ---
#
# fuzzy finder: Ctrl+R (history), Ctrl+T (files), Alt+C (directories)
if [[ -x "$(command -v fzf)" ]]; then
  eval "$(fzf --zsh)"
fi

# --- mise ---
#
if [[ -x "$(command -v mise)" ]]; then
  eval "$(mise activate zsh)"
fi

# --- os ---
#
# determine if linux or macos
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
  : # linux-specific aliases
elif [[ "$OSTYPE" == "darwin"* ]]; then
  # MacOS specific aliases
  alias tailscale='/Applications/Tailscale.app/Contents/MacOS/Tailscale'

  # MacOS specific PATHs
  export PATH=$PATH:/Applications/Firefox.app/Contents/MacOS # firefox
fi

# --- prompt ---
#
# set $ when user, # when root
if [[ $EUID -eq 0 ]]; then
  symbol='#'
else
  symbol='$'
fi
# embed cursor reset in prompt (use terminal default)
PROMPT=$(print "${PROMPT} \n %{\033[0 q%}${symbol} ")

# work - git worktree wrapper with cd support
work() {
  case "${1:-}" in
    -h|--help|help|ls|plan)
      command work "$@"
      return
      ;;
  esac
  local out
  out=$(command work "$@" | tail -1)
  if [[ -d "$out" ]]; then
    cd "$out"
  elif [[ -n "$out" ]]; then
    echo "$out"
  fi
}

# gcloud
if [ -f "$HOME/p/google-cloud-sdk/path.zsh.inc" ]; then . "$HOME/p/google-cloud-sdk/path.zsh.inc"; fi
if [ -f "$HOME/p/google-cloud-sdk/completion.zsh.inc" ]; then . "$HOME/p/google-cloud-sdk/completion.zsh.inc"; fi
