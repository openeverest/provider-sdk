package provider

import (
	"context"

	backupv1alpha1 "github.com/openeverest/openeverest/v2/api/backup/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller"
	"sigs.k8s.io/controller-runtime/pkg/client"
	// TODO: Import your operator's API types package, e.g.:
	// operatorv1 "github.com/example/my-operator/api/v1"
)

// TODO: If your operator supports operator-emitted backup CRs
// (typically produced by the wrapped operator's own scheduler,
// e.g. PSMDB BackupTask, pgBackRest cron, Barman scheduler) into
// first-class Backup CRs, implement this interface.
// This makes operator-scheduled backups visible via `kubectl get backups`
// and lets the runtime drive their lifecycle the same way as
// on-demand backups. All backup interfaces are optional.
//
// Compile-time interface checks.
var _ controller.BackupMirror = (*Provider)(nil)

// Mirror implements controller.BackupMirror (optional). The runtime invokes
// Mirror() for operator backup events. Return a Backup CR to create it
// idempotently, or nil to skip (on-demand backups, missing Instance, or backups
// when Instance has no backup configuration).
func (p *Provider) Mirror(ctx context.Context, c client.Client, obj client.Object) (*backupv1alpha1.Backup, error) {
	// TODO: Implement backup mirroring logic (optional).
	// Only needed if your operator creates backups independently (e.g., scheduled backups)
	// and you want them reflected in OpenEverest.
	//
	// Typical pattern:
	//   1. Check if obj is your operator backup type
	//   2. Check if this is a scheduled backup (not on-demand)
	//   3. Get the parent Instance CR
	//   4. Verify the Instance belongs to this provider
	//   5. Return a Backup CR to mirror the operator backup
	//
	// Example:
	//   ob, ok := obj.(*operatorv1.MyDatabaseBackup)
	//   if !ok {
	//       return nil, nil
	//   }
	//
	//   // TODO: check backup is produced by scheduled task
	//
	//   inst := &corev1alpha1.Instance{}
	//   err := c.Get(ctx, client.ObjectKey{Namespace: ob.Namespace, Name: ob.Spec.ClusterName}, inst)
	//   if err != nil || inst.Spec.Provider != p.Name() {
	//       return nil, nil
	//   }
	//
	//   return &backupv1alpha1.Backup{
	//       ObjectMeta: metav1.ObjectMeta{Name: ob.Name, Namespace: ob.Namespace},
	//       Spec: backupv1alpha1.BackupSpec{
	//           // TODO: set spec from your backup
	//       },
	//   }, nil

	return nil, nil
}

// OperatorBackupType implements controller.BackupMirror (optional).
func (p *Provider) OperatorBackupType() client.Object {
	// TODO: Return your operator's backup type.
	// Example: return &operatorv1.MyDatabaseBackup{}
	return nil
}
