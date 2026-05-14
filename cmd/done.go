package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/git"
	"github.com/zm1th/chipper/internal/manifest"
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

	// Gather file state before any interaction
	var userFiles, chipperFiles []git.FileStatus
	var gitRoot, branch string
	hasRemote := false

	if cfg.Git && !doneNoGit {
		if !git.IsInsideRepo() {
			return fmt.Errorf("not inside a git repository")
		}
		gitRoot, err = git.Root()
		if err != nil {
			return err
		}
		branch, err = git.CurrentBranch()
		if err != nil {
			return err
		}
		hasRemote = git.HasRemote()

		allFiles, err := git.ChangedFiles()
		if err != nil {
			return err
		}
		userFiles, chipperFiles = partitionFiles(allFiles, cfg.TicketsDir, gitRoot)
	}

	// --- Collect all decisions before making any changes ---

	var selectedAlso []string
	var selectedFiles []string
	var commitMsg string
	var push bool

	defaultMsg := fmt.Sprintf("chipper: done %s-%s", cfg.Project, slug)

	groups, err := buildDoneForm(
		cfg, entries, slug,
		userFiles, hasRemote, defaultMsg,
		&selectedAlso, &selectedFiles, &commitMsg, &push,
	)
	if err != nil {
		return err
	}

	if len(groups) > 0 {
		formErr := huh.NewForm(groups...).Run()
		if errors.Is(formErr, huh.ErrUserAborted) {
			fmt.Println("Aborted — no changes made.")
			return nil
		}
		if formErr != nil {
			return formErr
		}
	}

	// Append also-slugs from --also flag (non-interactive path)
	if alsoFlag != nil {
		for _, s := range alsoFlag {
			resolved := resolveSlug(s, cfg.Project)
			if manifest.FindBySlug(entries, resolved) == nil {
				return fmt.Errorf("ticket %q not found in queue", s)
			}
			selectedAlso = append(selectedAlso, resolved)
		}
	}

	// --- Apply all changes ---

	allDone := append([]string{slug}, selectedAlso...)
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
		// Auto-stage chipper-managed files
		if len(chipperFiles) > 0 {
			var paths []string
			for _, f := range chipperFiles {
				paths = append(paths, f.Path)
			}
			if err := git.StageFiles(paths); err != nil {
				return fmt.Errorf("failed to stage chipper files: %w", err)
			}
		}

		// Stage user-selected files
		if len(selectedFiles) > 0 {
			if err := git.StageFiles(selectedFiles); err != nil {
				return err
			}
		}

		if commitMsg == "" {
			commitMsg = defaultMsg
		}
		// Append any additionally closed tickets to commit message
		for _, s := range selectedAlso {
			commitMsg += fmt.Sprintf(", %s-%s", cfg.Project, s)
		}

		if err := git.Commit(commitMsg); err != nil {
			return err
		}
		fmt.Printf("Committed on branch %q\n", branch)

		if push {
			fmt.Println("Pushing...")
			if err := git.Push(branch); err != nil {
				return fmt.Errorf("push failed: %w", err)
			}
			fmt.Println("Pushed.")
		}
	}

	for _, s := range allDone {
		fmt.Printf("Done: %s-%s\n", cfg.Project, s)
	}
	return nil
}

func buildDoneForm(
	cfg *config.Config,
	entries []manifest.QueueEntry,
	currentSlug string,
	userFiles []git.FileStatus,
	hasRemote bool,
	defaultMsg string,
	selectedAlso *[]string,
	selectedFiles *[]string,
	commitMsg *string,
	push *bool,
) ([]*huh.Group, error) {
	var groups []*huh.Group

	// Group 1: other tickets to close (skip if --also flag used)
	if alsoFlag == nil {
		var opts []huh.Option[string]
		for _, e := range entries {
			if e.Slug == currentSlug || manifest.IsTerminal(e.Status) {
				continue
			}
			label := fmt.Sprintf("%s-%s  [%s]", cfg.Project, e.Slug, e.Status)
			opts = append(opts, huh.NewOption(label, e.Slug))
		}
		if len(opts) > 0 {
			groups = append(groups, huh.NewGroup(
				huh.NewMultiSelect[string]().
					Title("Any other tickets completed on this branch?").
					Description(fmt.Sprintf("Already included: %s-%s", cfg.Project, currentSlug)).
					Options(opts...).
					Value(selectedAlso),
			))
		}
	}

	// Group 2: file staging (only if there are user files)
	if len(userFiles) > 0 {
		fileOpts := make([]huh.Option[string], len(userFiles))
		var preselected []string
		for i, f := range userFiles {
			label := f.Path
			if f.Binary {
				label += "  [binary]"
			} else {
				preselected = append(preselected, f.Path)
			}
			fileOpts[i] = huh.NewOption(label, f.Path)
		}
		*selectedFiles = preselected
		*commitMsg = defaultMsg

		groups = append(groups, huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select files to stage").
				Options(fileOpts...).
				Value(selectedFiles),
			huh.NewText().
				Title("Commit message").
				Value(commitMsg),
		))
	}

	// Group 3: push confirmation (only if remote exists)
	if hasRemote {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title("Push branch to remote?").
				Value(push),
		))
	}

	return groups, nil
}

func partitionFiles(files []git.FileStatus, ticketsDir, gitRoot string) (user, chipper []git.FileStatus) {
	relTickets, err := filepath.Rel(gitRoot, ticketsDir)
	if err != nil {
		relTickets = ticketsDir
	}
	relTickets = filepath.ToSlash(relTickets)

	for _, f := range files {
		normalized := filepath.ToSlash(f.Path)
		if strings.HasPrefix(normalized, relTickets+"/") {
			chipper = append(chipper, f)
		} else {
			user = append(user, f)
		}
	}
	return
}
