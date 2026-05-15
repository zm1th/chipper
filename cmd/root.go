package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "chipper",
	Short: "File-based ticket management for your git repository",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Ticket lifecycle
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(archiveCmd)

	// Registration and sorting
	rootCmd.AddCommand(unregisteredCmd)
	rootCmd.AddCommand(orphanedCmd)
	rootCmd.AddCommand(unsortedCmd)
	rootCmd.AddCommand(sortCmd)

	// Setup and creation
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(newCmd)

	// Listing and browsing
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(topCmd)
	rootCmd.AddCommand(headCmd)
	rootCmd.AddCommand(showCmd)

	// list subcommands (read-only flag variants)
	listCmd.AddCommand(listUnregisteredCmd)
}
