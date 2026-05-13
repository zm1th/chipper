package cmd

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/git"
	"github.com/zm1th/chipper/internal/manifest"
	"github.com/zm1th/chipper/internal/ui"
)

var doneNoGit bool
var alsoFlag []string

var doneCmd = &cobra.Command{
	Use:   "done",
	Short: "Mark the in-progress ticket complete and commit changes",
	Args:  cobra.NoArgs,
	RunE:  runDone,
}

func init() {
	doneCmd.Flags().BoolVar(&doneNoGit, "no-git", false, "Skip git operations")
	doneCmd.Flags().StringSliceVar(&alsoFlag, "also", nil, "Additional slugs to mark done (comma-separated)")
}

func runDone(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	entries, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}

	inProgress := manifest.FindInProgress(entries)
	if inProgress == nil {
		return fmt.Errorf("no ticket is currently in progress")
	}

	slug := inProgress.Slug

	// Collect additional slugs to mark done
	also, err := resolveAlso(cfg, entries, slug)
	if err != nil {
		return err
	}

	// Mark all done
	allDone := append([]string{slug}, also...)
	for _, s := range allDone {
		entries, err = manifest.UpdateStatus(entries, s, "done")
		if err != nil {
			return err
		}
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
		return err
	}

	if cfg.Git && !doneNoGit {
		if !git.IsInsideRepo() {
			return fmt.Errorf("not inside a git repository")
		}

		files, err := git.ChangedFiles()
		if err != nil {
			return err
		}
		if len(files) == 0 {
			fmt.Println("No changed files — nothing to commit.")
		} else {
			branch, err := git.CurrentBranch()
			if err != nil {
				return err
			}

			defaultMsg := fmt.Sprintf("chipper: done %s-%s", cfg.Project, slug)
			if len(also) > 0 {
				for _, s := range also {
					defaultMsg += fmt.Sprintf(", %s-%s", cfg.Project, s)
				}
			}

			result, err := ui.RunStagingUI(files, defaultMsg, git.HasRemote())
			if err != nil {
				return err
			}

			if len(result.Files) == 0 {
				fmt.Println("No files staged — skipping commit.")
			} else {
				if err := git.StageFiles(result.Files); err != nil {
					return err
				}
				if err := git.Commit(result.Message); err != nil {
					return err
				}
				fmt.Printf("Committed on branch %q\n", branch)
			}

			if result.Push {
				fmt.Println("Pushing...")
				if err := git.Push(branch); err != nil {
					return fmt.Errorf("push failed: %w", err)
				}
				fmt.Println("Pushed.")
			}
		}
	}

	for _, s := range allDone {
		fmt.Printf("Done: %s-%s\n", cfg.Project, s)
	}
	return nil
}

// resolveAlso returns the list of additional slugs to mark done.
// If --also was passed, use that. Otherwise prompt interactively.
func resolveAlso(cfg *config.Config, entries []manifest.QueueEntry, currentSlug string) ([]string, error) {
	if alsoFlag != nil {
		// Resolve each slug and validate
		var resolved []string
		for _, s := range alsoFlag {
			slug := resolveSlug(s, cfg.Project)
			if manifest.FindBySlug(entries, slug) == nil {
				return nil, fmt.Errorf("ticket %q not found in queue", s)
			}
			resolved = append(resolved, slug)
		}
		return resolved, nil
	}

	// Build list of other active tickets for interactive selection
	var opts []huh.Option[string]
	for _, e := range entries {
		if e.Slug == currentSlug || manifest.IsTerminal(e.Status) {
			continue
		}
		label := fmt.Sprintf("%s-%s  [%s]", cfg.Project, e.Slug, e.Status)
		opts = append(opts, huh.NewOption(label, e.Slug))
	}

	if len(opts) == 0 {
		return nil, nil
	}

	var selected []string
	err := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().
			Title("Any other tickets completed on this branch?").
			Options(opts...).
			Value(&selected),
	)).Run()
	if err != nil {
		return nil, err
	}

	return selected, nil
}
