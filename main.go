package main

import (
	"context"
	"flag"
	"log/slog"
)

func main() {
	force := flag.Bool("force", false, "force overwrite of local files")
	flag.Parse()

	ctx := context.Background()

	slog.InfoContext(ctx, "starting dotfiles",
		"force", force,
	)
}
