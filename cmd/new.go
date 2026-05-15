package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/zm1th/chipper/internal/config"
	"github.com/zm1th/chipper/internal/manifest"
)

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new ticket, assign a slug, and sort it into the queue",
	Args:  cobra.ExactArgs(1),
	RunE:  runNew,
}

func runNew(_ *cobra.Command, args []string) error {
	name := args[0]

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ticketPath := filepath.Join(cfg.TicketsDir, name)
	if _, err := os.Stat(ticketPath); err == nil {
		return fmt.Errorf("ticket file %q already exists", name)
	}

	// Open editor or fall back to stdin
	content, err := collectTicketContent(ticketPath)
	if err != nil {
		return err
	}

	if err := os.WriteFile(ticketPath, []byte(content), 0644); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", ticketPath)

	// Load existing slugs and prompt for slug
	slugs, err := manifest.LoadSlugs(cfg.TicketsDir)
	if err != nil {
		return err
	}

	suggested := suggestSlug(name, slugs)
	var slug string

	err = huh.NewForm(huh.NewGroup(
		huh.NewInput().
			Title(fmt.Sprintf("Slug for %q", name)).
			Description(fmt.Sprintf("Suggested: %s  |  Project prefix: %s-", suggested, cfg.Project)).
			Placeholder(suggested).
			Validate(func(s string) error {
				if s == "" {
					return nil
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
	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Println("Aborted — ticket file created but not registered.")
		return nil
	}
	if err != nil {
		return err
	}

	if slug == "" {
		slug = suggested
	}

	if manifest.SlugTaken(slugs, slug) {
		return fmt.Errorf("slug %q is already taken", slug)
	}

	slugs[name] = slug

	queue, err := manifest.LoadQueue(cfg.TicketsDir)
	if err != nil {
		return err
	}
	queue = manifest.AddToQueue(queue, slug, "todo")

	if err := manifest.SaveSlugs(cfg.TicketsDir, slugs); err != nil {
		return err
	}
	if err := manifest.SaveQueue(cfg.TicketsDir, queue); err != nil {
		return err
	}

	fmt.Printf("Registered %q as %s-%s\n\n", name, cfg.Project, slug)

	return SortUnsorted(cfg)
}

// collectTicketContent opens $EDITOR if set, otherwise reads stdin until EOF.
func collectTicketContent(ticketPath string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}

	if editor != "" {
		// Write an empty file first so the editor has something to open
		if err := os.WriteFile(ticketPath, nil, 0644); err != nil {
			return "", err
		}
		cmd := exec.Command(editor, ticketPath)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("editor exited with error: %w", err)
		}
		data, err := os.ReadFile(ticketPath)
		if err != nil {
			return "", err
		}
		// Remove the file — caller will write it back
		_ = os.Remove(ticketPath)
		return string(data), nil
	}

	// No editor: read from stdin
	fmt.Println("Enter ticket content (Ctrl+D to finish):")
	var buf strings.Builder
	b := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.String(), nil
}
