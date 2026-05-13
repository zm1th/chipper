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

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List tickets",
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(doneCmd)
	rootCmd.AddCommand(cancelCmd)
	rootCmd.AddCommand(archiveCmd)
	rootCmd.AddCommand(unregisteredCmd)
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listUnregisteredCmd)
}
