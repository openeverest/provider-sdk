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

package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

// AddBackupConfig holds the configuration for adding backup support.
type AddBackupConfig struct {
	// IncludeMirror determines whether to also add backup_mirror.go
	IncludeMirror bool
}

// AddBackup adds backup implementation files to an existing provider project.
// It creates internal/provider/backup.go and optionally backup_mirror.go.
func AddBackup(cfg *AddBackupConfig) error {
	// Check if internal/provider exists.
	providerDir := filepath.Join("internal", "provider")
	if _, err := os.Stat(providerDir); err != nil {
		return fmt.Errorf("internal/provider directory not found; ensure you're in a provider project")
	}

	// Check if backup.go already exists.
	backupFile := filepath.Join(providerDir, "backup.go")
	if _, err := os.Stat(backupFile); err == nil {
		return fmt.Errorf("backup.go already exists at %s", backupFile)
	}

	// Create backup.go from template.
	if err := createBackupFile(backupFile); err != nil {
		return fmt.Errorf("creating backup.go: %w", err)
	}

	// Optionally create backup_mirror.go.
	if cfg.IncludeMirror {
		mirrorFile := filepath.Join(providerDir, "backup_mirror.go")
		if _, err := os.Stat(mirrorFile); err == nil {
			return fmt.Errorf("backup_mirror.go already exists at %s", mirrorFile)
		}
		if err := createBackupMirrorFile(mirrorFile); err != nil {
			return fmt.Errorf("creating backup_mirror.go: %w", err)
		}
	}

	return nil
}

func createBackupFile(path string) error {
	// Read template from embedded filesystem.
	templatePath := filepath.Join(templateRoot, "internal", "provider", "backup.go")
	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	// The template uses package provider, which is correct for the destination.
	// Just write it directly (no templating needed since imports are generic).
	return os.WriteFile(path, content, 0o644)
}

func createBackupMirrorFile(path string) error {
	// Read template from embedded filesystem.
	templatePath := filepath.Join(templateRoot, "internal", "provider", "backup_mirror.go")
	content, err := templateFS.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	return os.WriteFile(path, content, 0o644)
}
