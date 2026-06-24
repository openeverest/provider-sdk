package provider

import (
	backupv1alpha1 "github.com/openeverest/openeverest/v2/api/backup/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	// TODO: Import your operator's API types package, e.g.:
	// operatorv1 "github.com/example/my-operator/api/v1"
)

// TODO: If your operator supports backups and restores, implement the backup interfaces
// to enable OpenEverest's backup management. All backup interfaces are optional.
//
// Compile-time interface checks.
var _ controller.BackupProvider = (*Provider)(nil)
var _ controller.BackupWatcher = (*Provider)(nil)
var _ controller.RestoreWatcher = (*Provider)(nil)

// SyncBackup creates or updates the operator's backup resource, sets a controller
// reference from the Backup CR to enable owner-based watches, and maps operator
// status to OpenEverest states.
func (p *Provider) SyncBackup(c *controller.Context, backup *backupv1alpha1.Backup) (controller.BackupExecutionStatus, error) {
	l := log.FromContext(c.Context())
	l.Info("Syncing backup", "name", backup.Name)

	// TODO: Implement backup sync logic.
	// Typical pattern:
	//   1. Create or update the operator backup CR
	//   2. Set the backup spec (storage, cluster name, etc.)
	//   3. Set controller reference so the Backup owns the operator resource
	//   4. Map operator backup status to BackupExecutionStatus
	//
	// Example:
	//   ob := &operatorv1.MyDatabaseBackup{}
	//   if err := c.Get(ob, c.Name()); err != nil {
	//       return controller.BackupExecutionStatus{
	//           State:   backupv1alpha1.BackupStatePending,
	//           Message: "Waiting for backup to exist",
	//       }, nil
	//   }
	//
	//   if _, err := controllerutil.CreateOrUpdate(c.Context(), c.Client(), ob, func() error {
	//       // Set spec fields here
	//       return controllerutil.SetControllerReference(backup, ob, c.Client().Scheme())
	//   }); err != nil {
	//       return controller.BackupExecutionStatus{}, err
	//   }
	//
	//   exec := controller.BackupExecutionStatus{
	//       OperatorBackupRef: &corev1.TypedLocalObjectReference{
	//           APIGroup: pointer.ToString(operatorv1.SchemeGroupVersion.Group),
	//           Kind:     "MyDatabaseBackup",
	//           Name:     ob.Name,
	//       },
	//       State: backupv1alpha1.BackupStatePending,
	//   }
	//
	//   switch ob.Status.State {
	//   case "ready":
	//       exec.State = backupv1alpha1.BackupStateSucceeded
	//       exec.CompletedAt = pointer.To(metav1.Now())
	//   case "error":
	//       exec.State = backupv1alpha1.BackupStateFailed
	//       exec.Message = ob.Status.Error
	//   case "running":
	//       exec.State = backupv1alpha1.BackupStateRunning
	//   }
	//   return exec, nil

	return controller.BackupExecutionStatus{
		State: backupv1alpha1.BackupStatePending,
	}, nil
}

// SyncRestore resolves the source Backup CR, creates or updates the operator's
// restore resource with a controller reference, and maps operator status to
// OpenEverest states.
func (p *Provider) SyncRestore(c *controller.Context, restore *backupv1alpha1.Restore) (controller.RestoreExecutionStatus, error) {
	l := log.FromContext(c.Context())
	l.Info("Syncing restore", "name", restore.Name)

	// TODO: Implement restore sync logic.
	// Typical pattern:
	//   1. Get the source Backup CR
	//   2. Create or update the operator restore CR
	//   3. Set the restore spec (backup reference, cluster name, etc.)
	//   4. Set controller reference so the Restore owns the operator resource
	//   5. Map operator restore status to RestoreExecutionStatus
	//
	// Example:
	//   backup := &backupv1alpha1.Backup{}
	//   if err := c.Get(backup, restore.Spec.DataSource.BackupName); err != nil {
	//       return controller.RestoreExecutionStatus{
	//           State:   backupv1alpha1.RestoreStateFailed,
	//           Message: fmt.Sprintf("source Backup %q not found", restore.Spec.DataSource.BackupName),
	//       }, nil
	//   }
	//
	//   or := &operatorv1.MyDatabaseRestore{
	//       ObjectMeta: metav1.ObjectMeta{Name: restore.Name, Namespace: restore.Namespace},
	//   }
	//   if _, err := controllerutil.CreateOrUpdate(c.Context(), c.Client(), or, func() error {
	//       // Set spec fields here
	//       return controllerutil.SetControllerReference(restore, or, c.Client().Scheme())
	//   }); err != nil {
	//       return controller.RestoreExecutionStatus{}, err
	//   }
	//
	//   exec := controller.RestoreExecutionStatus{
	//       OperatorRestoreRef: &corev1.TypedLocalObjectReference{
	//           APIGroup: pointer.ToString(operatorv1.SchemeGroupVersion.Group),
	//           Kind:     "MyDatabaseRestore",
	//           Name:     or.Name,
	//       },
	//       State: backupv1alpha1.RestoreStatePending,
	//   }
	//
	//   switch or.Status.State {
	//   case "ready":
	//       exec.State = backupv1alpha1.RestoreStateSucceeded
	//       exec.CompletedAt = pointer.To(metav1.Now())
	//   case "error":
	//       exec.State = backupv1alpha1.RestoreStateFailed
	//       exec.Message = or.Status.Error
	//   case "running":
	//       exec.State = backupv1alpha1.RestoreStateRunning
	//   }
	//   return exec, nil

	return controller.RestoreExecutionStatus{
		State: backupv1alpha1.RestoreStatePending,
	}, nil
}

// CleanupBackup deletes the operator backup resource. For DeletionPolicy: Retain,
// remove storage-protection finalizers before deletion to preserve backup data.
// Return true only when fully deleted, false to requeue.
func (p *Provider) CleanupBackup(c *controller.Context, backup *backupv1alpha1.Backup) (bool, error) {
	l := log.FromContext(c.Context())
	l.Info("Cleaning up backup", "name", backup.Name)

	// TODO: Implement backup cleanup logic.
	// Typical pattern:
	//   1. Get the operator backup CR
	//   2. If DeletionPolicy is Retain, remove storage protection finalizers
	//   3. Delete the operator backup CR
	//   4. Return true when fully deleted, false to requeue
	//
	// Example:
	//   ob := &operatorv1.MyDatabaseBackup{}
	//   err := c.Get(ob, backup.Name)
	//   if apierrors.IsNotFound(err) {
	//       return true, nil
	//   }
	//   if err != nil {
	//       return false, err
	//   }
	//
	//   if backup.Spec.DeletionPolicy == backupv1alpha1.BackupDeletionPolicyRetain {
	//       // TODO: remove storage protection finalizer
	//   }
	//
	//   if ob.DeletionTimestamp.IsZero() {
	//       return false, c.Delete(ob)
	//   }
	//   return false, nil

	return true, nil
}

// CleanupRestore deletes the operator restore resource. Return true when fully
// deleted, false to requeue.
func (p *Provider) CleanupRestore(c *controller.Context, restore *backupv1alpha1.Restore) (bool, error) {
	l := log.FromContext(c.Context())
	l.Info("Cleaning up restore", "name", restore.Name)

	// TODO: Implement restore cleanup logic.
	// Typical pattern:
	//   1. Get the operator restore CR
	//   2. Delete the operator restore CR
	//   3. Return true when fully deleted, false to requeue
	//
	// Example:
	//   or := &operatorv1.MyDatabaseRestore{}
	//   err := c.Get(or, restore.Name)
	//   if apierrors.IsNotFound(err) {
	//       return true, nil
	//   }
	//   if err != nil {
	//       return false, err
	//   }
	//   if or.DeletionTimestamp.IsZero() {
	//       return false, c.Delete(or)
	//   }
	//   return false, nil

	return true, nil
}

// BackupWatches implements controller.BackupWatcher. Register watches so operator
// backup status changes trigger reconciliation. Use WatchOwned for resources with
// controller references set by SyncBackup.
func (p *Provider) BackupWatches() []controller.WatchConfig {
	// TODO: Register watches for your operator backup resource.
	// Example:
	//   return []controller.WatchConfig{
	//       controller.WatchOwned(&operatorv1.MyDatabaseBackup{}),
	//   }
	return []controller.WatchConfig{}
}

// RestoreWatches implements controller.RestoreWatcher. Register watches so operator
// restore status changes trigger reconciliation. Use WatchOwned for resources with
// controller references set by SyncRestore.
func (p *Provider) RestoreWatches() []controller.WatchConfig {
	// TODO: Register watches for your operator restore resource.
	// Example:
	//   return []controller.WatchConfig{
	//       controller.WatchOwned(&operatorv1.MyDatabaseRestore{}),
	//   }
	return []controller.WatchConfig{}
}
