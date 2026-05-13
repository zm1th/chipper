package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/git"
	"github.com/zm1th/chipper/internal/manifest"
)

var startNoGit bool

var startCmd = &cobra.Command{
	Use:   "start <id>",
	Short: "Mark a ticket in progress and create a git branch",
	Args:  cobra.ExactArgs(1),
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVar(&startNoGit, "no-git", false, "Skip git operations")
}

func runStart(_ *cobra.Command, args []string) error {
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

	if inProgress := manifest.FindInProgress(entries); inProgress != nil {
		return fmt.Errorf("ticket %q is already in progress — complete or cancel it first", inProgress.Slug)
	}

	if cfg.Git && !startNoGit {
		if !git.IsInsideRepo() {
			return fmt.Errorf("not inside a git repository")
		}

		onTrunk, err := git.IsOnBranch(cfg.TrunkBranch)
		if err != nil {
			return err
		}
		if !onTrunk {
			return fmt.Errorf("must be on %q branch to start a ticket", cfg.TrunkBranch)
		}

		if git.HasRemote() {
			upToDate, err := git.IsUpToDate(cfg.TrunkBranch)
			if err != nil {
				return err
			}
			if !upToDate {
				return fmt.Errorf("%q is behind remote — pull before starting a ticket", cfg.TrunkBranch)
			}
		}

		branch := ticketBranch(cfg, slug)
		fmt.Printf("Creating branch %q...\n", branch)
		if err := git.CreateAndCheckoutBranch(branch); err != nil {
			return fmt.Errorf("failed to create branch: %w", err)
		}
	}

	entries, err = manifest.UpdateStatus(entries, slug, "in_progress")
	if err != nil {
		return err
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
		return err
	}

	if cfg.Git && !startNoGit {
		queuePath := cfg.TicketsDir + "/chipper-queue"
		if err := git.StageFiles([]string{queuePath}); err != nil {
			return err
		}
		if err := git.Commit(fmt.Sprintf("chipper: start %s-%s", cfg.Project, slug)); err != nil {
			return err
		}
	}

	fmt.Printf("Started %s-%s\n", cfg.Project, slug)
	return nil
}

func resolveSlug(id, project string) string {
	prefix := strings.ToUpper(project) + "-"
	if strings.HasPrefix(strings.ToUpper(id), prefix) {
		return id[len(prefix):]
	}
	return id
}

func ticketBranch(cfg *config.Config, slug string) string {
	name := fmt.Sprintf("%s-%s", cfg.Project, slug)
	if cfg.BranchPrefix != "" {
		return cfg.BranchPrefix + "/" + name
	}
	return name
}
