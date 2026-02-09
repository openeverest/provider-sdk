package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	goversion "github.com/hashicorp/go-version"
	"github.com/percona/everest-operator/api/everest/v1alpha1/dataimporterspec"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	everestv1alpha1 "github.com/percona/everest-operator/api/everest/v1alpha1"
	psmdbv1 "github.com/percona/percona-server-mongodb-operator/pkg/apis/psmdb/v1"

	v2alpha1 "github.com/openeverest/provider-sdk/pkg/apis/v2alpha1"
)

const (
	// DeletePSMDBBackupFinalizer is the finalizer used to ensure proper cleanup of PSMDB Cluster backups.
	// This matches the finalizer used in everest-operator (percona.com/delete-backup).
	DeletePSMDBBackupFinalizer = "percona.com/delete-backup"
	// LegacyDeleteBackupFinalizer is the old finalizer that needs to be replaced.
	LegacyDeleteBackupFinalizer = "delete-backup"
)

// BackupCmd is the command for running backup operations.
var BackupCmd = &cobra.Command{
	Use:  "backup",
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		configPath := args[0]
		if err := runBackup(cmd.Context(), configPath); err != nil {
			log.Printf("Failed to run backup: %v", err)
			panic(err)
		}
	},
}

func runBackup(ctx context.Context, configPath string) error {
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

	if cfg.Source.BackupStorageName == nil {
		return fmt.Errorf("source backupStorageName is required")
	}

	var (
		dbName            = cfg.Target.DatabaseClusterRef.Name
		namespace         = cfg.Target.DatabaseClusterRef.Namespace
		backupStorageName = *cfg.Source.BackupStorageName
	)

	// prepare k8s client.
	k8sClient, err := client.New(config.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return err
	}

	// Fetch the BackupStorage CR to get S3 configuration
	backupStorage := &everestv1alpha1.BackupStorage{}
	if err := k8sClient.Get(ctx, client.ObjectKey{
		Name:      backupStorageName,
		Namespace: namespace,
	}, backupStorage); err != nil {
		return fmt.Errorf("failed to get BackupStorage %s: %w", backupStorageName, err)
	}

	var (
		bucket   = backupStorage.Spec.Bucket
		endpoint = backupStorage.Spec.EndpointURL
		region   = backupStorage.Spec.Region
	)

	// FIXME: obviously we can't have a hardcoded name, I just wanted to make
	// the tests work quickly
	psmdbBackupName := "test-db-backup-s3"

	log.Printf("Starting PSMDB backup for database %s in namespace %s", dbName, namespace)

	// Get the PSMDB cluster CR to check version and configuration
	psmdbCluster := &psmdbv1.PerconaServerMongoDB{}
	if err := k8sClient.Get(ctx, types.NamespacedName{
		Name:      dbName,
		Namespace: namespace,
	}, psmdbCluster); err != nil {
		return fmt.Errorf("failed to get PSMDB cluster %s: %w", dbName, err)
	}

	// Check if backup configuration is ready (for PSMDB 1.20.0+)
	versionCheck, err := isCRVersionGreaterOrEqual(psmdbCluster.Spec.CRVersion, "1.20.0")
	if err != nil {
		return fmt.Errorf("failed to compare CR version for PSMDB cluster %s: %w", dbName, err)
	}

	if versionCheck && psmdbCluster.Status.BackupConfigHash == "" {
		log.Printf("Backup configuration is not ready yet for PSMDB cluster %s, waiting...", dbName)
		// Wait for backup configuration to be ready
		if err := waitForBackupConfigReady(ctx, k8sClient, namespace, dbName); err != nil {
			return fmt.Errorf("failed waiting for backup configuration: %w", err)
		}
		log.Printf("Backup configuration is ready for PSMDB cluster %s", dbName)
	}

	// Wait for the PSMDB CR to have the storage configured that matches our source.
	log.Printf("Waiting for PSMDB cluster %s to have storage configured", dbName)
	storageName, err := waitForStorageConfiguration(ctx, k8sClient, namespace, dbName, bucket, endpoint, region)
	if err != nil {
		return fmt.Errorf("failed to find matching storage configuration: %w", err)
	}
	log.Printf("Found matching storage %s in PSMDB cluster %s", storageName, dbName)

	// Get the BackupJob CR that owns this Job by traversing owner references
	backupJob, err := getBackupJobOwner(ctx, k8sClient, namespace)
	if err != nil {
		return fmt.Errorf("failed to get BackupJob owner: %w", err)
	}
	log.Printf("Found BackupJob owner: %s", backupJob.GetName())

	// Create and wait for the PSMDB backup to complete.
	log.Printf("Starting PSMDB backup %s for database %s to storage %s", psmdbBackupName, dbName, storageName)
	if err := runPSMDBBackupAndWait(ctx, k8sClient, namespace, dbName, psmdbBackupName, storageName, backupJob); err != nil {
		return fmt.Errorf("failed to run PSMDB backup: %w", err)
	}
	log.Printf("PSMDB backup %s completed successfully for database %s", psmdbBackupName, dbName)
	return nil
}

// isCRVersionGreaterOrEqual checks if the current version is greater than or equal to the desired version.
func isCRVersionGreaterOrEqual(currentVersionStr, desiredVersionStr string) (bool, error) {
	crVersion, err := goversion.NewVersion(currentVersionStr)
	if err != nil {
		return false, err
	}
	desiredVersion, err := goversion.NewVersion(desiredVersionStr)
	if err != nil {
		return false, err
	}
	return crVersion.GreaterThanOrEqual(desiredVersion), nil
}

// getBackupJobOwner finds the BackupJob CR that owns this job by traversing owner references.
// It gets the current pod, then the Job that owns it, then the BackupJob that owns the Job.
func getBackupJobOwner(ctx context.Context, c client.Client, namespace string) (*v2alpha1.BackupJob, error) {
	// Get the current pod using the hostname (pod name)
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		return nil, fmt.Errorf("HOSTNAME environment variable not set")
	}

	pod := &corev1.Pod{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      podName,
		Namespace: namespace,
	}, pod); err != nil {
		return nil, fmt.Errorf("failed to get current pod %s: %w", podName, err)
	}

	// Find the Job owner from the pod's owner references
	var jobOwnerRef *metav1.OwnerReference
	for i := range pod.OwnerReferences {
		if pod.OwnerReferences[i].Kind == "Job" && pod.OwnerReferences[i].APIVersion == "batch/v1" {
			jobOwnerRef = &pod.OwnerReferences[i]
			break
		}
	}
	if jobOwnerRef == nil {
		return nil, fmt.Errorf("pod %s is not owned by a Job", podName)
	}

	// Get the Job
	job := &batchv1.Job{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      jobOwnerRef.Name,
		Namespace: namespace,
	}, job); err != nil {
		return nil, fmt.Errorf("failed to get Job %s: %w", jobOwnerRef.Name, err)
	}

	// Find the BackupJob owner from the job's owner references
	var backupJobOwnerRef *metav1.OwnerReference
	for i := range job.OwnerReferences {
		if job.OwnerReferences[i].Kind == "BackupJob" {
			backupJobOwnerRef = &job.OwnerReferences[i]
			break
		}
	}
	if backupJobOwnerRef == nil {
		return nil, fmt.Errorf("job %s is not owned by a BackupJob", job.Name)
	}

	// Get the BackupJob
	backupJob := &v2alpha1.BackupJob{}
	if err := c.Get(ctx, types.NamespacedName{
		Name:      backupJobOwnerRef.Name,
		Namespace: namespace,
	}, backupJob); err != nil {
		return nil, fmt.Errorf("failed to get BackupJob %s: %w", backupJobOwnerRef.Name, err)
	}

	return backupJob, nil
}

// waitForBackupConfigReady waits for the PSMDB backup configuration to be ready (BackupConfigHash is set).
func waitForBackupConfigReady(
	ctx context.Context,
	c client.Client,
	namespace, dbName string,
) error {
	retryInterval := 5 * time.Second
	return wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		psmdb := &psmdbv1.PerconaServerMongoDB{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      dbName,
			Namespace: namespace,
		}, psmdb); err != nil {
			return false, fmt.Errorf("failed to get PSMDB cluster %s: %w", dbName, err)
		}
		return psmdb.Status.BackupConfigHash != "", nil
	})
}

// waitForStorageConfiguration waits for the PSMDB CR to have a storage configured that matches the source configuration.
// Returns the storage name when found.
func waitForStorageConfiguration(
	ctx context.Context,
	c client.Client,
	namespace, dbName, bucket, endpoint, region string,
) (string, error) {
	retryInterval := 5 * time.Second
	var storageName string

	err := wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		psmdb := &psmdbv1.PerconaServerMongoDB{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      dbName,
			Namespace: namespace,
		}, psmdb); err != nil {
			return false, fmt.Errorf("failed to get PSMDB cluster %s: %w", dbName, err)
		}

		// Check if backup is enabled
		if !psmdb.Spec.Backup.Enabled {
			return false, nil
		}

		// Look for a storage that matches our source configuration
		for name, storage := range psmdb.Spec.Backup.Storages {
			if storage.Type != psmdbv1.BackupStorageS3 {
				continue
			}
			// Match by bucket, endpoint, and region
			if storage.S3.Bucket == bucket &&
				storage.S3.EndpointURL == endpoint &&
				storage.S3.Region == region {
				storageName = name
				return true, nil
			}
		}

		return false, nil
	})

	if err != nil {
		return "", err
	}
	return storageName, nil
}

// runPSMDBBackupAndWait creates a PSMDB backup CR and waits for it to complete.
func runPSMDBBackupAndWait(
	ctx context.Context,
	c client.Client,
	namespace, dbName, psmdbBackupName, storageName string,
	backupJob *v2alpha1.BackupJob,
) error {
	psmdbBackup := &psmdbv1.PerconaServerMongoDBBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      psmdbBackupName,
			Namespace: namespace,
		},
	}

	// Check if backup already exists
	err := c.Get(ctx, client.ObjectKey{
		Name:      psmdbBackupName,
		Namespace: namespace,
	}, psmdbBackup)

	if err == nil {
		// Backup already exists
		// Wait for it to progress beyond the waiting state before updating
		// This is a known limitation in PSMDB operator, where updating the object while it is in the waiting
		// state results in a duplicate backup being created.
		// See: https://perconadev.atlassian.net/browse/K8SPSMDB-1088
		if !ptr.To(psmdbBackup.GetCreationTimestamp()).IsZero() &&
			(psmdbBackup.Status.State == "" || psmdbBackup.Status.State == psmdbv1.BackupStateWaiting) {
			log.Printf("PSMDB backup %s is in waiting state, waiting for it to progress...", psmdbBackupName)
			if err := waitForBackupToProgressPastWaiting(ctx, c, namespace, psmdbBackupName); err != nil {
				return fmt.Errorf("failed waiting for backup to progress past waiting state: %w", err)
			}
			log.Printf("PSMDB backup %s has progressed past waiting state", psmdbBackupName)
		}
	}

	// CreateOrUpdate the backup
	_, err = controllerutil.CreateOrUpdate(ctx, c, psmdbBackup, func() error {
		psmdbBackup.Spec.ClusterName = dbName
		psmdbBackup.Spec.StorageName = storageName

		// Replace legacy finalizer with the proper one (mimics reconcilePSMDB)
		controllerutil.RemoveFinalizer(psmdbBackup, LegacyDeleteBackupFinalizer)
		controllerutil.AddFinalizer(psmdbBackup, DeletePSMDBBackupFinalizer)

		// Set owner reference to the BackupJob CR that triggered this backup
		return controllerutil.SetControllerReference(backupJob, psmdbBackup, c.Scheme())
	})

	if err != nil {
		return fmt.Errorf("failed to create or update PSMDB backup: %w", err)
	}
	log.Printf("PSMDB backup %s created/updated successfully", psmdbBackupName)

	// Wait for the backup to complete
	log.Printf("Waiting for PSMDB backup %s to complete", psmdbBackupName)
	retryInterval := 5 * time.Second
	return wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		psmdbBackup := &psmdbv1.PerconaServerMongoDBBackup{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      psmdbBackupName,
			Namespace: namespace,
		}, psmdbBackup); err != nil {
			return false, fmt.Errorf("failed to get PSMDB backup %s: %w", psmdbBackupName, err)
		}

		// Check for error state
		if psmdbBackup.Status.State == psmdbv1.BackupStateError {
			return false, fmt.Errorf("PSMDB backup failed with error: %s", psmdbBackup.Status.Error)
		}

		// Check if backup is ready
		return psmdbBackup.Status.State == psmdbv1.BackupStateReady, nil
	})
}

// waitForBackupToProgressPastWaiting waits for the backup to progress beyond the waiting state.
func waitForBackupToProgressPastWaiting(
	ctx context.Context,
	c client.Client,
	namespace, psmdbBackupName string,
) error {
	retryInterval := 5 * time.Second
	return wait.PollUntilContextCancel(ctx, retryInterval, true, func(ctx context.Context) (bool, error) {
		psmdbBackup := &psmdbv1.PerconaServerMongoDBBackup{}
		if err := c.Get(ctx, client.ObjectKey{
			Name:      psmdbBackupName,
			Namespace: namespace,
		}, psmdbBackup); err != nil {
			return false, fmt.Errorf("failed to get PSMDB backup %s: %w", psmdbBackupName, err)
		}

		// Continue waiting if still in waiting or empty state
		if psmdbBackup.Status.State == "" || psmdbBackup.Status.State == psmdbv1.BackupStateWaiting {
			return false, nil
		}

		return true, nil
	})
}
