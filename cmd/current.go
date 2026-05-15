package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var currentCmd = &cobra.Command{
	Use:   "current",
	Short: "Display the ticket currently in progress",
	Args:  cobra.NoArgs,
	RunE:  runCurrent,
}

func runCurrent(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	inProgress := manifest.FindInProgress(queue)
	if inProgress == nil {
		fmt.Println("No ticket is currently in progress.")
		return nil
	}

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}

	filename := manifest.SlugToFile(slugs)[inProgress.Slug]
	if filename == "" {
		return fmt.Errorf("no file found for ticket %s-%s", cfg.Project, inProgress.Slug)
	}

	data, err := os.ReadFile(filepath.Join(cfg.TicketsDir, filename))
	if err != nil {
		return err
	}

	fmt.Printf("=== %s-%s (%s) ===\n\n", cfg.Project, inProgress.Slug, filename)
	fmt.Print(string(data))
	return nil
}
