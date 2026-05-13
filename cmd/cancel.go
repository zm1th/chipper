package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var cancelCmd = &cobra.Command{
	Use:   "cancel <id>",
	Short: "Mark a ticket as cancelled",
	Args:  cobra.ExactArgs(1),
	RunE:  runCancel,
}

func runCancel(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slug := resolveSlug(args[0], cfg.Project)

	entries, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	if manifest.FindBySlug(entries, slug) == nil {
		return fmt.Errorf("ticket %q not found in queue", args[0])
	}

	entries, err = manifest.UpdateStatus(entries, slug, "cancelled")
	if err != nil {
		return err
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
		return err
	}

	fmt.Printf("Cancelled %s-%s\n", cfg.Project, slug)
	return nil
}
