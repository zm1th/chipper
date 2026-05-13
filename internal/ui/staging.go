package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/zm1th/chipper/internal/git"
)

type StagingResult struct {
	Files   []string
	Message string
	Push    bool
}

func RunStagingUI(files []git.FileStatus, defaultMessage string, hasPush bool) (*StagingResult, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("no changed files to commit")
	}

	opts := make([]huh.Option[string], len(files))
	var preselected []string
	for i, f := range files {
		label := f.Path
		if f.Binary {
			label += "  [binary]"
		} else {
			preselected = append(preselected, f.Path)
		}
		opts[i] = huh.NewOption(label, f.Path)
	}

	selected := preselected
	message := defaultMessage
	var push bool

	groups := []*huh.Group{
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select files to stage").
				Options(opts...).
				Value(&selected),
			huh.NewText().
				Title("Commit message").
				Value(&message),
		),
	}

	if hasPush {
		groups = append(groups, huh.NewGroup(
			huh.NewConfirm().
				Title("Push branch to remote?").
				Value(&push),
		))
	}

	if err := huh.NewForm(groups...).Run(); err != nil {
		return nil, err
	}

	return &StagingResult{
		Files:   selected,
		Message: message,
		Push:    push,
	}, nil
}
