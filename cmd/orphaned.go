package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var orphanedCmd = &cobra.Command{
	Use:   "orphaned",
	Short: "List orphaned tickets and interactively relink each to an existing file",
	Args:  cobra.NoArgs,
	RunE:  runOrphaned,
}

func runOrphaned(_ *cobra.Command, _ []string) error {
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

	orphaned := manifest.OrphanedSlugs(cfg.TicketsDir, slugs, queue)
	if len(orphaned) == 0 {
		fmt.Println("No orphaned tickets.")
		return nil
	}

	// Load unregistered files as candidates for relinking
	unregistered, err := manifest.UnregisteredFiles(cfg.TicketsDir)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d orphaned ticket(s).\n\n", len(orphaned))

	for _, slug := range orphaned {
		fmt.Printf("Orphaned: %s-%s\n", cfg.Project, slug)

		var newFilename string

		if len(unregistered) > 0 {
			// Offer unregistered files as quick options
			opts := make([]huh.Option[string], len(unregistered)+1)
			opts[0] = huh.NewOption("Enter manually", "__manual__")
			for i, f := range unregistered {
				opts[i+1] = huh.NewOption(f, f)
			}

			var choice string
			err := huh.NewForm(huh.NewGroup(
				huh.NewSelect[string]().
					Title(fmt.Sprintf("Relink %s-%s to which file?", cfg.Project, slug)).
					Description("These unregistered files may be renames of this ticket.").
					Options(opts...).
					Value(&choice),
			)).Run()
			if err != nil {
				return err
			}

			if choice != "__manual__" {
				newFilename = choice
			}
		}

		if newFilename == "" {
			err := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title(fmt.Sprintf("New filename for %s-%s (blank to skip)", cfg.Project, slug)).
					Validate(func(s string) error {
						if s == "" {
							return nil
						}
						if strings.ContainsAny(s, "/\\") {
							return fmt.Errorf("filename must not contain slashes")
						}
						return nil
					}).
					Value(&newFilename),
			)).Run()
			if err != nil {
				return err
			}
		}

		if newFilename == "" {
			fmt.Printf("Skipped %s-%s\n\n", cfg.Project, slug)
			continue
		}

		// Update chipper-slugs: remove old entry (if any), add new
		for filename, s := range slugs {
			if s == slug {
				delete(slugs, filename)
				break
			}
		}
		slugs[newFilename] = slug

		// Remove from unregistered candidates
		for i, f := range unregistered {
			if f == newFilename {
				unregistered = append(unregistered[:i], unregistered[i+1:]...)
				break
			}
		}

		fmt.Printf("Relinked %s-%s → %s\n\n", cfg.Project, slug, newFilename)
	}

	return manifest.SaveSlugs(cfg.TicketsDir, slugs)
}
