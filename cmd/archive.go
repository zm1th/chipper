package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var archiveCmd = &cobra.Command{
	Use:   "archive <id>",
	Short: "Archive a ticket, removing it from the active queue",
	Args:  cobra.ExactArgs(1),
	RunE:  runArchive,
}

func runArchive(_ *cobra.Command, args []string) error {
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

	entries, err = manifest.UpdateStatus(entries, slug, "archived")
	if err != nil {
		return err
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
		return err
	}

	fmt.Printf("Archived %s-%s\n", cfg.Project, slug)
	return nil
}
