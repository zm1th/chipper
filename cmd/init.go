package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var (
	initProject string
	initDir     string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new chipper project in the current directory",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func init() {
	initCmd.Flags().StringVar(&initProject, "project", "", "Project identifier (e.g. CHP)")
	initCmd.Flags().StringVar(&initDir, "dir", "", "Tickets directory name")
}

func runInit(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	chipperFile := filepath.Join(cwd, ".chipper")
	alreadyExists := false
	if _, err := os.Stat(chipperFile); err == nil {
		alreadyExists = true
	}

	project := initProject
	dir := initDir

	if project == "" || dir == "" {
		var confirmOverwrite bool

		var fields []huh.Field

		if alreadyExists {
			fields = append(fields, huh.NewConfirm().
				Title(".chipper already exists — overwrite it?").
				Value(&confirmOverwrite))
		}
		if project == "" {
			fields = append(fields, huh.NewInput().
				Title("Project identifier").
				Description("Short uppercase prefix for ticket IDs, e.g. CHP").
				Placeholder("CHP").
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("project identifier is required")
					}
					if strings.ContainsAny(s, " \t/\\") {
						return fmt.Errorf("project identifier must not contain spaces or slashes")
					}
					return nil
				}).
				Value(&project))
		}
		if dir == "" {
			fields = append(fields, huh.NewInput().
				Title("Tickets directory").
				Placeholder("chipper-tickets").
				Value(&dir))
		}

		formErr := huh.NewForm(huh.NewGroup(fields...)).Run()
		if errors.Is(formErr, huh.ErrUserAborted) {
			fmt.Println("Aborted.")
			return nil
		}
		if formErr != nil {
			return formErr
		}

		if alreadyExists && !confirmOverwrite {
			fmt.Println("Keeping existing .chipper — no changes made.")
			return nil
		}
	}

	if strings.TrimSpace(project) == "" {
		project = "CHP"
	}
	if strings.TrimSpace(dir) == "" {
		dir = "chipper-tickets"
	}
	project = strings.ToUpper(strings.TrimSpace(project))

	// Write .chipper config
	content := fmt.Sprintf("project = %s\ntickets_dir = %s\ngit = true\ntrunk_branch = main\nbranch_prefix =\n", project, dir)
	if err := os.WriteFile(chipperFile, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", chipperFile)

	// Create tickets directory
	ticketsPath := filepath.Join(cwd, dir)
	if err := os.MkdirAll(ticketsPath, 0755); err != nil {
		return err
	}
	fmt.Printf("Created %s/\n", dir)

	// Create empty manifest files
	for _, name := range []string{"chipper-slugs", "chipper-queue"} {
		p := filepath.Join(ticketsPath, name)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if err := os.WriteFile(p, nil, 0644); err != nil {
				return err
			}
			fmt.Printf("Created %s/%s\n", dir, name)
		}
	}

	fmt.Printf("\nInitialized chipper project %q in %s\n", project, cwd)
	return nil
}
