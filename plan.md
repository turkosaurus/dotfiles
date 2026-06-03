# TODO

## declarative tmux
1. workspace
  - (~) gh ci
  - (~) today.md
    - pomo hello
  - ~/p/workspace
2. ~/dotfiles
3. ~/w

## health
- [ ] count the number of clicks and keystrokes for the last 1, 5, 15 minutes and 1, 8, 24 hours

## tmux inactive pane greyscale — dispatched 2026-06-02 (worker)

**Brief:** Investigate how to render non-active tmux panes in greyscale, propose a fix to `~/dotfiles/home/.tmux.conf`.

**Status:** done

<!-- worker:results-start -->
### What tmux supports

- `window-style` styles **inactive** panes; `window-active-style` styles the
  focused pane. Attributes: `fg=`, `bg=`, plus `dim`, `bold`, `reverse`,
  `underscore`.
- tmux **cannot desaturate** arbitrary cell content — it can only override
  fg/bg or apply terminal attributes like `dim`. Anything already colored by
  the running program (vim syntax, ls colors) keeps its color unless you
  force `fg=` and the program's output respects it (most don't — they emit
  SGR sequences that override `window-style`).
- The `dim` SGR attribute is the closest thing to "greyscale": terminals
  that honor it (kitty, alacritty, iterm2, wezterm) render half-intensity.
- Your existing `pane-border-style` only colors borders, not pane contents
  — that's a separate setting.

### Options, simplest → most invasive

1. **`dim` attribute on inactive panes** — most elegant; preserves the
   tokyonight palette on the active pane, terminal renders inactive panes
   at half brightness. Doesn't truly desaturate, but reads as muted.
2. **Force grey fg** (`fg=colour245`) — guaranteed visual change, but
   programs emitting their own SGR colors will override it.
3. **Darker bg** (`bg=#16161e` or similar) — subtle inactive shade; works
   regardless of program output.
4. **Combine `dim` + grey fg + slight bg shift** — strongest effect, but
   busy and may fight tokyonight on the active pane.

True greyscale (desaturating colors *as rendered*) is not possible at the
tmux layer — it'd need a terminal-level shader (e.g. kitty's
`background_tint`/custom shader, or picom on X11).

### Proposed diff

Insert after line 55 in `~/dotfiles/home/.tmux.conf`:

```diff
 set -g pane-border-lines heavy
 set -g pane-border-style "fg=#{@secondary}"            # inactive
 set -g pane-active-border-style "fg=#{@primary},bold"  # active
+
+# Dim inactive panes (terminal half-intensity, reads as greyscale)
+set -g window-style "fg=colour245,dim"
+set -g window-active-style "default"
```

Try option 1 first — if `dim` is barely visible in your terminal, layer in
a bg shift: `set -g window-style "fg=colour245,bg=#16161e,dim"`.
<!-- worker:results-end -->
