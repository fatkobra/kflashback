// Copyright 2024 The kflashback Authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	flashbackv1alpha1 "github.com/kflashback/kflashback/api/v1alpha1"
	"github.com/kflashback/kflashback/internal/storage"
)

// FlashbackPolicyReconciler reconciles FlashbackPolicy objects.
type FlashbackPolicyReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	DynClient dynamic.Interface
	Store     storage.Store
	Watcher   *ResourceWatcher
}

// +kubebuilder:rbac:groups=flashback.io,resources=flashbackpolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=flashback.io,resources=flashbackpolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=flashback.io,resources=flashbackpolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups="*",resources="*",verbs=get;list;watch

// Reconcile handles FlashbackPolicy changes.
func (r *FlashbackPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy flashbackv1alpha1.FlashbackPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("FlashbackPolicy deleted, stopping watchers", "name", req.Name)
			r.Watcher.StopPolicy(req.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	logger.Info("reconciling FlashbackPolicy", "name", policy.Name, "resources", len(policy.Spec.Resources))

	// Reconcile resource watchers
	if err := r.Watcher.Reconcile(ctx, &policy); err != nil {
		logger.Error(err, "failed to reconcile watchers")
		r.setCondition(&policy, "Ready", metav1.ConditionFalse, "WatcherError", err.Error())
		r.updateStatus(ctx, &policy)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	// Run retention cleanup
	if err := r.runRetention(ctx, &policy); err != nil {
		logger.Error(err, "failed to run retention cleanup")
	}

	// Update status
	if err := r.refreshStatus(ctx, &policy); err != nil {
		logger.Error(err, "failed to refresh status")
	}

	r.setCondition(&policy, "Ready", metav1.ConditionTrue, "Reconciled", fmt.Sprintf("Tracking %d resource types", len(policy.Spec.Resources)))
	if policy.Spec.Paused {
		r.setCondition(&policy, "Ready", metav1.ConditionFalse, "Paused", "Policy is paused")
	}

	if err := r.updateStatus(ctx, &policy); err != nil {
		logger.Error(err, "failed to update status")
		return ctrl.Result{}, err
	}

	// Requeue periodically for retention cleanup
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// runRetention purges old revisions based on the retention policy.
func (r *FlashbackPolicyReconciler) runRetention(ctx context.Context, policy *flashbackv1alpha1.FlashbackPolicy) error {
	maxAge := policy.Spec.Retention.MaxAge
	if maxAge == "" {
		maxAge = "720h"
	}

	duration, err := time.ParseDuration(maxAge)
	if err != nil {
		return fmt.Errorf("parsing maxAge %q: %w", maxAge, err)
	}

	cutoff := time.Now().Add(-duration)
	purged, err := r.Store.PurgeOldRevisions(ctx, cutoff)
	if err != nil {
		return err
	}

	if purged > 0 {
		log.FromContext(ctx).Info("purged old revisions", "count", purged, "cutoff", cutoff)
	}

	return nil
}

// refreshStatus updates the policy status with current statistics.
func (r *FlashbackPolicyReconciler) refreshStatus(ctx context.Context, policy *flashbackv1alpha1.FlashbackPolicy) error {
	stats, err := r.Store.GetStats(ctx)
	if err != nil {
		return err
	}

	policy.Status.TrackedResources = int32(stats.TotalResources)
	policy.Status.TotalRevisions = stats.TotalRevisions
	policy.Status.StorageUsedBytes = stats.StorageBytes
	policy.Status.StorageUsed = formatBytes(stats.StorageBytes)
	now := metav1.Now()
	policy.Status.LastReconcileTime = &now

	kindStats, err := r.Store.GetKindStats(ctx)
	if err != nil {
		return err
	}

	policy.Status.ResourceSummaries = make([]flashbackv1alpha1.ResourceSummary, len(kindStats))
	for i, ks := range kindStats {
		policy.Status.ResourceSummaries[i] = flashbackv1alpha1.ResourceSummary{
			APIVersion: ks.APIVersion,
			Kind:       ks.Kind,
			Count:      int32(ks.Resources),
			Revisions:  ks.Revisions,
		}
	}

	return nil
}

func (r *FlashbackPolicyReconciler) updateStatus(ctx context.Context, policy *flashbackv1alpha1.FlashbackPolicy) error {
	return r.Status().Update(ctx, policy)
}

func (r *FlashbackPolicyReconciler) setCondition(policy *flashbackv1alpha1.FlashbackPolicy, condType string, status metav1.ConditionStatus, reason, message string) {
	meta.SetStatusCondition(&policy.Status.Conditions, metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	})
}

// SetupWithManager sets up the controller with the Manager.
func (r *FlashbackPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&flashbackv1alpha1.FlashbackPolicy{}).
		Complete(r)
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
