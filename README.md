# dotfiles

Configuration and setup scripts.

## first use
Clone the repo, then `source init.sh` to setup.

```bash
source <(wget -qO- https://raw.githubusercontent.com/turkosaurus/dotfiles/refs/heads/main/init.sh)
```
- [home/*](home) is symlinked to `$HOME`
- [home/bin](home/bin) binaries are added to path (can be invoked directly)

## updating

[`dotsync`](home/bin/dotsync) symlinks dotfiles in a `stow`-like fashion.

Normal usage updates all files from remote and adds symlinks.
```bash
dotsync
```

For a verbose (`-v`), local-only (`-l`) run:
```bash
dotsync -vl
```

