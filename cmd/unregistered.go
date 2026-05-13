package cmd

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var unregisteredCmd = &cobra.Command{
	Use:   "unregistered",
	Short: "List unregistered ticket files and interactively assign slugs",
	Args:  cobra.NoArgs,
	RunE:  runUnregistered,
}

var listUnregisteredCmd = &cobra.Command{
	Use:   "unregistered",
	Short: "List ticket files not yet assigned a slug",
	Args:  cobra.NoArgs,
	RunE:  runListUnregistered,
}

func runListUnregistered(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	files, err := manifest.UnregisteredFiles(cfg.TicketsDir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("No unregistered ticket files.")
		return nil
	}

	for _, f := range files {
		fmt.Println(f)
	}
	return nil
}

func runUnregistered(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	files, err := manifest.UnregisteredFiles(cfg.TicketsDir)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		fmt.Println("No unregistered ticket files.")
		return nil
	}

	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}

	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	fmt.Printf("Found %d unregistered file(s). Assign a slug to each, or leave blank to skip.\n\n", len(files))

	for _, filename := range files {
		suggested := suggestSlug(filename, slugs)
		var slug string

		err := huh.NewForm(huh.NewGroup(
			huh.NewInput().
				Title(fmt.Sprintf("Slug for %q", filename)).
				Description(fmt.Sprintf("Suggested: %s  |  Project prefix: %s-", suggested, cfg.Project)).
				Placeholder(suggested).
				Validate(func(s string) error {
					if s == "" {
						return nil // blank = skip
					}
					if strings.ContainsAny(s, " \t/\\") {
						return fmt.Errorf("slug must not contain spaces or slashes")
					}
					if manifest.SlugTaken(slugs, s) {
						return fmt.Errorf("slug %q is already taken", s)
					}
					return nil
				}).
				Value(&slug),
		)).Run()
		if err != nil {
			return err
		}

		if slug == "" {
			slug = suggested
			if manifest.SlugTaken(slugs, slug) {
				fmt.Printf("Skipped %q (suggested slug %q already taken)\n", filename, slug)
				continue
			}
		}

		slugs[filename] = slug
		queue = manifest.AddToQueue(queue, slug, "todo")
		fmt.Printf("Registered %q as %s-%s\n", filename, cfg.Project, slug)
	}

	if err := manifest.SaveSlugs(cfg.TicketsDir, slugs); err != nil {
		return err
	}
	return manifest.SaveQueue(cfg.TicketsDir, queue)
}

// suggestSlug derives a slug from a filename, trimming common extensions.
func suggestSlug(filename string, existing map[string]string) string {
	// Strip extension if present
	name := filename
	if idx := strings.LastIndex(name, "."); idx > 0 {
		name = name[:idx]
	}
	// Replace non-slug chars with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)
	name = strings.ToLower(name)

	if !manifest.SlugTaken(existing, name) {
		return name
	}

	// Append a number until unique
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", name, i)
		if !manifest.SlugTaken(existing, candidate) {
			return candidate
		}
	}
}
