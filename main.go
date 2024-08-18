package main

import (
	"context"
	"log/slog"
	"flag"
)

func main() {
	force := flag.Bool("force", false, "force overwrite of local files")
	flag.Parse()

	slog.InfoConext(ctx, "starting dotfiles",
		"force", force,
)
}