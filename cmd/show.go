package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var showCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Display the full content of a ticket",
	Args:  cobra.ExactArgs(1),
	RunE:  runShow,
}

func runShow(_ *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slug := resolveSlug(args[0], cfg.Project)

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}

	slugToFile := manifest.SlugToFile(slugs)
	filename, ok := slugToFile[slug]
	if !ok {
		return fmt.Errorf("no ticket found for slug %q", args[0])
	}

	data, err := os.ReadFile(filepath.Join(cfg.TicketsDir, filename))
	if err != nil {
		return err
	}

	fmt.Printf("=== %s-%s (%s) ===\n\n", cfg.Project, slug, filename)
	fmt.Print(string(data))
	return nil
}
