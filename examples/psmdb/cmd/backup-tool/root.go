package main

import (
	"github.com/spf13/cobra"
)

// Root command for running backup-tool commands.
var Root = &cobra.Command{
	Use: "backup-tool",
}

func init() {
	Root.AddCommand(BackupCmd)
	Root.AddCommand(CleanupCmd)
}
