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

var (
	doneNoGit    bool
	alsoFlag     []string
	doneMessage  string
	doneAllFiles bool
	donePushFlag bool
	doneNoPush   bool
)

var doneCmd = &cobra.Command{
	Use:   "done",
	Short: "Mark the in-progress ticket complete and commit changes",
	Args:  cobra.NoArgs,
	RunE:  runDone,
}

func init() {
	doneCmd.Flags().BoolVar(&doneNoGit, "no-git", false, "Skip git operations")
	doneCmd.Flags().StringSliceVar(&alsoFlag, "also", nil, "Additional slugs to mark done (comma-separated)")
	doneCmd.Flags().StringVarP(&doneMessage, "message", "m", "", "Commit message (skips interactive prompt)")
	doneCmd.Flags().BoolVar(&doneAllFiles, "all-files", false, "Stage all changed non-binary files (skips file selection)")
	doneCmd.Flags().BoolVar(&donePushFlag, "push", false, "Push after committing")
	doneCmd.Flags().BoolVar(&doneNoPush, "no-push", false, "Skip push after committing")
}

func runDone(cmd *cobra.Command, _ []string) error {
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
	defaultMsg := fmt.Sprintf("chipper: done %s-%s", cfg.Project, slug)

	// Resolve --also slugs early so we can validate before any interaction
	var selectedAlso []string
	if alsoFlag != nil {
		for _, s := range alsoFlag {
			resolved := resolveSlug(s, cfg.Project)
			if manifest.FindBySlug(entries, resolved) == nil {
				return fmt.Errorf("ticket %q not found in queue", s)
			}
			selectedAlso = append(selectedAlso, resolved)
		}
	}

	// ── Git pre-flight ────────────────────────────────────────────────────────

	var userFiles []git.FileStatus
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
		userFiles = excludeChipperFiles(allFiles, cfg.TicketsDir, gitRoot)
	}

	// ── Collect decisions ─────────────────────────────────────────────────────

	// Flags that were explicitly passed (not just their default values)
	messageFlagged := cmd.Flags().Lookup("message").Changed
	pushFlagged := donePushFlag || doneNoPush

	// When message, files, and push are all covered by flags, treat as fully
	// non-interactive: skip the "also" group too.
	alsoProvided := alsoFlag != nil || (messageFlagged && doneAllFiles && pushFlagged)

	// Decisions — start from flag values, fill gaps interactively
	selectedFiles := flagSelectedFiles(userFiles) // may be overridden by form
	commitMsg := doneMessage
	push := donePushFlag

	groups := buildDoneGroups(
		cfg, entries, slug,
		userFiles, hasRemote,
		defaultMsg,
		alsoProvided, messageFlagged, doneAllFiles, pushFlagged,
		&selectedAlso, &selectedFiles, &commitMsg, &push,
	)

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

	// ── Build final commit message ────────────────────────────────────────────

	if commitMsg == "" {
		commitMsg = defaultMsg
	}
	for _, s := range selectedAlso {
		commitMsg += fmt.Sprintf(", %s-%s", cfg.Project, s)
	}

	allDone := append([]string{slug}, selectedAlso...)

	// ── Apply ─────────────────────────────────────────────────────────────────

	if cfg.Git && !doneNoGit {
		onTrunk, err := git.IsOnBranch(cfg.TrunkBranch)
		if err != nil {
			return err
		}
		if onTrunk {
			return fmt.Errorf("on trunk branch %q — run chipper start to begin a ticket branch before finishing", cfg.TrunkBranch)
		}

		if err := applyAndCommit(cfg, entries, allDone, selectedFiles, commitMsg, branch, push); err != nil {
			return err
		}
	} else {
		for _, s := range allDone {
			entries, err = manifest.UpdateStatus(entries, s, "done")
			if err != nil {
				return err
			}
		}
		if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
			return err
		}
	}

	for _, s := range allDone {
		fmt.Printf("Done: %s-%s\n", cfg.Project, s)
	}

	// Skip the trunk-switch prompt when running non-interactively
	nonInteractive := messageFlagged || doneAllFiles || pushFlagged
	if cfg.Git && !doneNoGit && !nonInteractive {
		if err := promptSwitchToTrunk(cfg.TrunkBranch, push); err != nil {
			return err
		}
	}

	return nil
}

// flagSelectedFiles returns all non-binary user files when --all-files is set,
// otherwise returns nil (caller will fill via form or leave empty).
func flagSelectedFiles(userFiles []git.FileStatus) []string {
	if !doneAllFiles {
		return nil
	}
	var out []string
	for _, f := range userFiles {
		if !f.Binary {
			out = append(out, f.Path)
		}
	}
	return out
}

// buildDoneGroups returns only the form groups needed for decisions not already
// covered by flags.
func buildDoneGroups(
	cfg *config.Config,
	entries []manifest.QueueEntry,
	currentSlug string,
	userFiles []git.FileStatus,
	hasRemote bool,
	defaultMsg string,
	alsoProvided, messageProvided, allFilesProvided, pushProvided bool,
	selectedAlso *[]string,
	selectedFiles *[]string,
	commitMsg *string,
	push *bool,
) []*huh.Group {
	var groups []*huh.Group

	// Group 1: other tickets to close
	if !alsoProvided {
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

	// Group 2: file staging + commit message
	needFileSelect := len(userFiles) > 0 && !allFilesProvided
	needMessage := !messageProvided

	if needFileSelect || needMessage {
		var fields []huh.Field

		if needFileSelect {
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
			fields = append(fields, huh.NewMultiSelect[string]().
				Title("Select files to stage").
				Options(fileOpts...).
				Value(selectedFiles))
		}

		if needMessage {
			*commitMsg = defaultMsg
			fields = append(fields, huh.NewText().
				Title("Commit message").
				Value(commitMsg))
		}

		groups = append(groups, huh.NewGroup(fields...))
	}

	// Group 3: push confirmation
	if hasRemote && !pushProvided {
		*push = true
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title("Push branch to remote?").
				Value(push),
		))
	}

	return groups
}

func promptSwitchToTrunk(trunk string, pushed bool) error {
	switchToTrunk := pushed
	err := huh.NewForm(huh.NewGroup(
		huh.NewConfirm().
			Title(fmt.Sprintf("Switch back to %q?", trunk)).
			Value(&switchToTrunk),
	)).Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return nil
	}
	if err != nil {
		return err
	}
	if switchToTrunk {
		if err := git.Checkout(trunk); err != nil {
			return fmt.Errorf("failed to switch to %q: %w", trunk, err)
		}
		fmt.Printf("Switched to %q\n", trunk)
	}
	return nil
}

func applyAndCommit(cfg *config.Config, entries []manifest.QueueEntry, allDone, selectedFiles []string, commitMsg, branch string, push bool) error {
	var err error
	for _, s := range allDone {
		entries, err = manifest.UpdateStatus(entries, s, "done")
		if err != nil {
			return err
		}
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, entries); err != nil {
		return err
	}

	if err := git.StageFiles([]string{cfg.TicketsDir}); err != nil {
		return fmt.Errorf("failed to stage chipper files: %w", err)
	}
	if len(selectedFiles) > 0 {
		if err := git.StageFiles(selectedFiles); err != nil {
			return err
		}
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
	return nil
}

func excludeChipperFiles(files []git.FileStatus, ticketsDir, gitRoot string) []git.FileStatus {
	relTickets, err := filepath.Rel(gitRoot, ticketsDir)
	if err != nil {
		relTickets = ticketsDir
	}
	relTickets = filepath.ToSlash(relTickets)

	var user []git.FileStatus
	for _, f := range files {
		if !strings.HasPrefix(filepath.ToSlash(f.Path), relTickets+"/") {
			user = append(user, f)
		}
	}
	return user
}
