package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Project      string
	TicketsDir   string
	Git          bool
	BranchPrefix string
	TrunkBranch  string
	RootDir      string
}

func Load() (*Config, error) {
	dir, err := findRoot()
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		TicketsDir:  "chipper-tickets",
		Git:         true,
		TrunkBranch: "main",
		RootDir:     dir,
	}

	f, err := os.Open(filepath.Join(dir, ".chipper"))
	if err != nil {
		return nil, fmt.Errorf("no .chipper file found — run chipper init")
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "project":
			cfg.Project = val
		case "tickets_dir":
			cfg.TicketsDir = val
		case "git":
			cfg.Git = val == "true"
		case "branch_prefix":
			cfg.BranchPrefix = val
		case "trunk_branch":
			cfg.TrunkBranch = val
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if cfg.Project == "" {
		return nil, fmt.Errorf("project not set in .chipper")
	}

	if !filepath.IsAbs(cfg.TicketsDir) {
		cfg.TicketsDir = filepath.Join(dir, cfg.TicketsDir)
	}

	return cfg, nil
}

func findRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, ".chipper")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no .chipper file found in this directory or any parent — run chipper init")
		}
		dir = parent
	}
}
