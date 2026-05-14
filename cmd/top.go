package cmd

import (
	"strconv"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var topCmd = &cobra.Command{
	Use:   "top [n]",
	Short: "Show the top n active tickets by priority (default 5)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runTop,
}

func runTop(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	n := 5
	if len(args) == 1 {
		if n, err = strconv.Atoi(args[0]); err != nil {
			return err
		}
	}

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}
	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	return printTop(cfg, queue, slugs, n)
}
