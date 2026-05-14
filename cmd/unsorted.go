package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var unsortedCmd = &cobra.Command{
	Use:   "unsorted",
	Short: "List active tickets not yet placed in the priority queue",
	Args:  cobra.NoArgs,
	RunE:  runUnsorted,
}

func runUnsorted(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	unsorted := manifest.UnsortedEntries(queue)
	if len(unsorted) == 0 {
		fmt.Println("No unsorted tickets.")
		return nil
	}

	for _, e := range unsorted {
		fmt.Printf("%s-%s  [%s]\n", cfg.Project, e.Slug, e.Status)
	}
	return nil
}
