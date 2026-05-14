package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var headCmd = &cobra.Command{
	Use:   "head",
	Short: "Display the full content of the highest-priority active ticket",
	Args:  cobra.NoArgs,
	RunE:  runHead,
}

func runHead(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}
	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	head := manifest.Head(queue)
	if head == nil {
		fmt.Println("No active tickets.")
		return nil
	}

	slugToFile := manifest.SlugToFile(slugs)
	filename := slugToFile[head.Slug]
	if filename == "" {
		return fmt.Errorf("no file found for ticket %s-%s", cfg.Project, head.Slug)
	}

	data, err := os.ReadFile(filepath.Join(cfg.TicketsDir, filename))
	if err != nil {
		return err
	}

	fmt.Printf("=== %s-%s (%s) ===\n\n", cfg.Project, head.Slug, filename)
	fmt.Print(string(data))
	return nil
}
