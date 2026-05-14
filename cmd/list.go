package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
	"github.com/zm1th/chipper/internal/ui"
)

var (
	listTopN     int
	listUnsorted bool
	listOrphaned bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Browse tickets interactively, or list a filtered subset",
	RunE:  runList,
}

func init() {
	listCmd.Flags().IntVar(&listTopN, "top", 0, "Show top N active tickets (default 5 when used without a value)")
	// Allow --top with no value, defaulting to 5
	listCmd.Flags().Lookup("top").NoOptDefVal = "5"
	listCmd.Flags().BoolVar(&listUnsorted, "unsorted", false, "Show unsorted tickets")
	listCmd.Flags().BoolVar(&listOrphaned, "orphaned", false, "Show orphaned tickets")
}

func runList(_ *cobra.Command, _ []string) error {
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

	switch {
	case listTopN > 0:
		return printTop(cfg, queue, slugs, listTopN)
	case listUnsorted:
		return printUnsorted(cfg, queue)
	case listOrphaned:
		return printOrphaned(cfg, slugs, queue)
	default:
		return runInteractiveList(cfg, slugs, queue)
	}
}

func runInteractiveList(cfg *config.Config, slugs map[string]string, queue []manifest.QueueEntry) error {
	slugToFile := manifest.SlugToFile(slugs)
	entries := buildTicketEntries(cfg, queue, slugToFile)
	return ui.RunTicketList(entries, cfg.Project)
}

func buildTicketEntries(cfg *config.Config, queue []manifest.QueueEntry, slugToFile map[string]string) []ui.TicketEntry {
	var entries []ui.TicketEntry
	for _, e := range queue {
		filename := slugToFile[e.Slug]
		content := ""
		if filename != "" {
			data, err := os.ReadFile(filepath.Join(cfg.TicketsDir, filename))
			if err == nil {
				content = string(data)
			}
		}
		entries = append(entries, ui.TicketEntry{
			Slug:     e.Slug,
			Filename: filename,
			Status:   e.Status,
			Index:    e.Index,
			Content:  content,
		})
	}
	return entries
}

func printTop(cfg *config.Config, queue []manifest.QueueEntry, slugs map[string]string, n int) error {
	slugToFile := manifest.SlugToFile(slugs)
	top := manifest.TopN(queue, n)
	if len(top) == 0 {
		fmt.Println("No active tickets.")
		return nil
	}
	for _, e := range top {
		filename := slugToFile[e.Slug]
		fmt.Printf("%s-%s  %s\n", cfg.Project, e.Slug, filename)
	}
	return nil
}

func printUnsorted(cfg *config.Config, queue []manifest.QueueEntry) error {
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

func printOrphaned(cfg *config.Config, slugs map[string]string, queue []manifest.QueueEntry) error {
	orphaned := manifest.OrphanedSlugs(cfg.TicketsDir, slugs, queue)
	if len(orphaned) == 0 {
		fmt.Println("No orphaned tickets.")
		return nil
	}
	for _, slug := range orphaned {
		fmt.Printf("%s-%s\n", cfg.Project, slug)
	}
	return nil
}

// printList prints all tickets in queue order (used by non-interactive fallback).
func printList(cfg *config.Config, queue []manifest.QueueEntry, slugToFile map[string]string) {
	for _, e := range queue {
		filename := slugToFile[e.Slug]
		indexStr := "     "
		if e.Index > 0 {
			indexStr = fmt.Sprintf("%-5d", e.Index)
		}
		fmt.Printf("%s  %s-%s  %s  %s\n",
			indexStr, cfg.Project, e.Slug,
			strings.ToLower(e.Status),
			filename,
		)
	}
}
