# dotfiles

Configuration and setup scripts.

## first use
Clone the repo, then run [`dotsync`](home/bin/dotsync) to symlink all dotfiles in a `stow`-like fasion.

```bash
git clone https://github.com/turkosaurus/dotfiles
chmod +x dotfiles/home/bin/*/*
./dotfiles/home/bin/dotsync -v
```

## updating
All files in [home/bin](home/bin) are added to path.

For a verbose (`-v`), local-only (`-l`) run:
```bash
dotsync -vl
```

