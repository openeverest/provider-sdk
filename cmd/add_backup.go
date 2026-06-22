// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openeverest/provider-sdk/internal/scaffold"
)

var addBackupOpts struct {
	includeMirror bool
}

func init() {
	f := addBackupCmd.Flags()
	f.BoolVar(&addBackupOpts.includeMirror, "include-mirror", false, "Also add backup_mirror.go for operator-scheduled backup mirroring")

	addCmd.AddCommand(addBackupCmd)
}

var addBackupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Add backup support to the provider",
	Long: `Add backup implementation files to an existing provider project.

This command creates the following files:
  - internal/provider/backup.go        (SyncBackup and SyncRestore implementations)
  - internal/provider/backup_mirror.go (optional: Mirror for operator-scheduled backups)

The backup.go file contains stub implementations of the BackupProvider interface
with comprehensive TODO comments. The backup_mirror.go file is only created if
--include-mirror is specified, and implements the BackupMirror interface for
reflecting operator-emitted backups as OpenEverest Backup CRs.

Run from the provider project root directory.

Examples:
  # Add basic backup support
  provider-sdk add backup

  # Add backup support with mirroring for operator-scheduled backups
  provider-sdk add backup --include-mirror

After adding backup support:
  1. Implement backup logic in internal/provider/backup.go
  2. If mirroring: implement Mirror logic in internal/provider/backup_mirror.go`,
	RunE: runAddBackup,
}

func runAddBackup(_ *cobra.Command, _ []string) error {
	fmt.Println()

	cfg := &scaffold.AddBackupConfig{
		IncludeMirror: addBackupOpts.includeMirror,
	}
	if err := scaffold.AddBackup(cfg); err != nil {
		return fmt.Errorf("adding backup support: %w", err)
	}

	fmt.Println()
	fmt.Println("=== Backup support added successfully! ===")
	fmt.Println()
	fmt.Println("Created files:")
	fmt.Println("  - internal/provider/backup.go")
	if cfg.IncludeMirror {
		fmt.Println("  - internal/provider/backup_mirror.go")
	}
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Implement backup logic in internal/provider/backup.go")
	if cfg.IncludeMirror {
		fmt.Println("  2. Implement Mirror logic in internal/provider/backup_mirror.go")
	}
	fmt.Println()

	return nil
}
