// Copyright (c) 2026, NVIDIA CORPORATION.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package job

import (
	"context"
	"log/slog"
	"strings"

	"github.com/NVIDIA/aicr/pkg/errors"
	"github.com/NVIDIA/aicr/pkg/validator/labels"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// ServiceAccountName is the name of the ServiceAccount used by all validator Jobs.
	ServiceAccountName = "aicr-validator"

	// ClusterRoleName is the name of the purpose-built ClusterRole for validators.
	ClusterRoleName = "aicr-validator"

	// ClusterRoleBindingName is the name of the ClusterRoleBinding that grants
	// the validator ClusterRole to the validator ServiceAccount.
	ClusterRoleBindingName = "aicr-validator"

	// fieldManager is the SSA field manager name for all RBAC resources.
	fieldManager = labels.ValueAICR
)

// EnsureRBAC applies the ServiceAccount, ClusterRole, and ClusterRoleBinding
// for validator Jobs using server-side apply. Call once per validation run
// before deploying any Jobs.
func EnsureRBAC(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	if err := applyServiceAccount(ctx, clientset, namespace); err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to apply ServiceAccount", err)
	}

	if err := applyClusterRole(ctx, clientset); err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to apply ClusterRole", err)
	}

	if err := applyClusterRoleBinding(ctx, clientset, namespace); err != nil {
		return errors.Wrap(errors.ErrCodeInternal, "failed to apply ClusterRoleBinding", err)
	}

	slog.Debug("RBAC resources applied",
		"serviceAccount", ServiceAccountName,
		"namespace", namespace,
		"clusterRole", ClusterRoleName)

	return nil
}

// CleanupRBAC removes the ServiceAccount, ClusterRole, and ClusterRoleBinding.
// Ignores NotFound errors (idempotent). Call once at end of validation run.
func CleanupRBAC(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	var errs []string

	if err := clientset.CoreV1().ServiceAccounts(namespace).Delete(ctx, ServiceAccountName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, "ServiceAccount: "+err.Error())
		}
	}

	if err := clientset.RbacV1().ClusterRoleBindings().Delete(ctx, ClusterRoleBindingName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, "ClusterRoleBinding: "+err.Error())
		}
	}

	if err := clientset.RbacV1().ClusterRoles().Delete(ctx, ClusterRoleName, metav1.DeleteOptions{}); err != nil {
		if !apierrors.IsNotFound(err) {
			errs = append(errs, "ClusterRole: "+err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(errors.ErrCodeInternal, "RBAC cleanup failed: "+strings.Join(errs, "; "))
	}

	slog.Debug("RBAC resources cleaned up",
		"serviceAccount", ServiceAccountName,
		"namespace", namespace)

	return nil
}

func applyServiceAccount(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	sa := applycorev1.ServiceAccount(ServiceAccountName, namespace).
		WithLabels(map[string]string{
			labels.Name:      labels.ValueValidator,
			labels.ManagedBy: labels.ValueAICR,
		})

	_, err := clientset.CoreV1().ServiceAccounts(namespace).Apply(
		ctx, sa, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}

func applyClusterRole(ctx context.Context, clientset kubernetes.Interface) error {
	cr := applyrbacv1.ClusterRole(ClusterRoleName).
		WithLabels(map[string]string{
			labels.Name:      labels.ValueValidator,
			labels.ManagedBy: labels.ValueAICR,
		}).
		WithRules(
			applyrbacv1.PolicyRule().
				WithAPIGroups("").
				WithResources("pods", "pods/log", "services", "configmaps", "namespaces", "nodes", "serviceaccounts", "events").
				WithVerbs("get", "list", "watch"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("").
				WithResources("pods").
				WithVerbs("create", "delete"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("").
				WithResources("secrets").
				WithVerbs("get", "list"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("apps").
				WithResources("deployments", "daemonsets", "statefulsets", "replicasets").
				WithVerbs("get", "list", "watch"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("batch").
				WithResources("jobs").
				WithVerbs("get", "list", "watch"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("autoscaling").
				WithResources("horizontalpodautoscalers").
				WithVerbs("get", "list", "watch", "create", "delete"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("karpenter.sh").
				WithResources("nodepools", "nodeclaims").
				WithVerbs("get", "list"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("resource.k8s.io").
				WithResources("resourceclaims", "resourceclaimtemplates", "deviceclasses").
				WithVerbs("get", "list", "create", "delete"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("gateway.networking.k8s.io").
				WithResources("gateways", "httproutes").
				WithVerbs("get", "list"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("monitoring.coreos.com").
				WithResources("servicemonitors", "podmonitors").
				WithVerbs("get", "list"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("scheduling.x-k8s.io").
				WithResources("podgroups").
				WithVerbs("get", "list", "create", "delete"),
			applyrbacv1.PolicyRule().
				WithAPIGroups("rbac.authorization.k8s.io").
				WithResources("clusterroles", "clusterrolebindings", "roles", "rolebindings").
				WithVerbs("get", "list"),
		)

	_, err := clientset.RbacV1().ClusterRoles().Apply(
		ctx, cr, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}

func applyClusterRoleBinding(ctx context.Context, clientset kubernetes.Interface, namespace string) error {
	crb := applyrbacv1.ClusterRoleBinding(ClusterRoleBindingName).
		WithLabels(map[string]string{
			labels.Name:      labels.ValueValidator,
			labels.ManagedBy: labels.ValueAICR,
		}).
		WithSubjects(
			applyrbacv1.Subject().
				WithKind("ServiceAccount").
				WithName(ServiceAccountName).
				WithNamespace(namespace),
		).
		WithRoleRef(
			applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(ClusterRoleName),
		)

	_, err := clientset.RbacV1().ClusterRoleBindings().Apply(
		ctx, crb, metav1.ApplyOptions{FieldManager: fieldManager, Force: true},
	)
	return err
}
