package main

import (
	"context"
	"fmt"
	"log"

	v2alpha1 "github.com/openeverest/provider-sdk/pkg/apis/v2alpha1"
	"github.com/percona/everest-operator/api/everest/v1alpha1/dataimporterspec"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	everestv1alpha1 "github.com/percona/everest-operator/api/everest/v1alpha1"
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"
)

const (
	// psmdbStorageProtectionFinalizer is the finalizer added by PSMDB operator to protect backup storage.
	psmdbStorageProtectionFinalizer = "percona.com/delete-backup"
)

// CleanupCmd is the command for running cleanup operations.
var CleanupCmd = &cobra.Command{
	Use:  "cleanup",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath := args[0]
		if err := runCleanup(cmd.Context(), configPath); err != nil {
			log.Printf("Failed to run cleanup: %v", err)
			panic(err)
		}
	},
}

func runCleanup(ctx context.Context, configPath string) error {
	cfg := &dataimporterspec.Spec{}
	if err := cfg.ReadFromFilepath(configPath); err != nil {
		return err
	}

	// prepare API scheme.
	scheme := runtime.NewScheme()
	if err := everestv1alpha1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add everestv1alpha1 to scheme: %w", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add corev1 to scheme: %w", err)
	}
	if err := psmdbv1.SchemeBuilder.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add psmdbv1 to scheme: %w", err)
	}
	if err := v2alpha1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add v2alpha1 to scheme: %w", err)
	}
	if err := batchv1.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add batchv1 to scheme: %w", err)
	}

	var (
		dbName    = cfg.Target.DatabaseClusterRef.Name
		namespace = cfg.Target.DatabaseClusterRef.Namespace
	)

	// prepare k8s client.
	k8sClient, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	// FIXME: obviously we can't have a hardcoded name, I just wanted to make
	// the tests work quickly
	psmdbBackupName := "test-db-backup-s3"

	log.Printf("Starting cleanup for PSMDB backup %s in namespace %s", psmdbBackupName, namespace)

	// Get the PSMDB backup object
	psmdbBackup := &psmdbv1.PerconaServerMongoDBBackup{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      psmdbBackupName,
		Namespace: namespace,
	}, psmdbBackup); err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.Printf("PSMDB backup %s not found, nothing to cleanup", psmdbBackupName)
			return nil
		}
		return fmt.Errorf("failed to get PSMDB backup %s: %w", psmdbBackupName, err)
	}

	// Get the BackupJob CR that owns this Job by traversing owner references
	backupJob, err := getBackupJobOwner(ctx, k8sClient, namespace)
	if err != nil {
		return fmt.Errorf("failed to get BackupJob owner: %w", err)
	}
	log.Printf("Found BackupJob owner: %s", backupJob.GetName())

	// Check if the BackupJob has the storage protection finalizer. If not, there's nothing to do.
	if !controllerutil.ContainsFinalizer(backupJob, everestv1alpha1.DBBackupStorageProtectionFinalizer) {
		return nil
	}
	// Ensure that S3 finalizer is removed from the upstream backup.
	if controllerutil.RemoveFinalizer(psmdbBackup, psmdbStorageProtectionFinalizer) {
		if err := k8sClient.Update(ctx, psmdbBackup); err != nil {
			return fmt.Errorf("failed to remove finalizer from PSMDB backup %s: %w", psmdbBackupName, err)
		}
	}
	// Finalizer is gone from upstream object, remove from DatabaseClusterBackup.
	if controllerutil.RemoveFinalizer(backupJob, everestv1alpha1.DBBackupStorageProtectionFinalizer) {
		if err := k8sClient.Update(ctx, backupJob); err != nil {
			return fmt.Errorf("failed to remove finalizer from BackupJob %s: %w", backupJob.GetName(), err)
		}
	}

	log.Printf("Cleanup completed successfully for database %s", dbName)
	return nil
}
