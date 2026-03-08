package diagnostics

import (
	"context"
	"fmt"

	"github.com/topiaruss/kogaro/ui/pkg/graph"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// diagnoseDanglingReference checks if the referenced resource exists and suggests similar resources.
func diagnoseDanglingReference(ctx context.Context, c client.Client, err graph.ErrorDetail) ([]DiagnosticFinding, error) {
	var findings []DiagnosticFinding

	// Extract the referenced resource name from the error message
	targetName := extractQuotedName(err.Message)
	if targetName == "" {
		targetName = err.ResourceName
	}

	// Determine the target kind from the error code
	targetKind := refCodeToKind(err.ErrorCode)

	findings = append(findings, DiagnosticFinding{
		Category: "labels",
		Summary:  fmt.Sprintf("%s %s/%s references %s '%s'", err.ResourceType, err.Namespace, err.ResourceName, targetKind, targetName),
		Details: map[string]string{
			"source":      err.ResourceName,
			"target":      targetName,
			"target_kind": targetKind,
		},
	})

	// Try to verify the target doesn't exist
	exists, existErr := resourceExists(ctx, c, err.Namespace, targetName, targetKind)
	if existErr == nil {
		if exists {
			findings = append(findings, DiagnosticFinding{
				Category: "labels",
				Summary:  fmt.Sprintf("%s '%s' now exists — this error may be stale", targetKind, targetName),
				Details:  map[string]string{"status": "resolved"},
			})
		} else {
			findings = append(findings, DiagnosticFinding{
				Category: "labels",
				Summary:  fmt.Sprintf("%s '%s' does not exist in namespace %s", targetKind, targetName, err.Namespace),
				Details:  map[string]string{"status": "missing"},
			})
		}
	}

	// List similar resources of the same kind
	similar := listSimilarResources(ctx, c, err.Namespace, targetKind)
	if len(similar) > 0 {
		names := ""
		for i, name := range similar {
			if i > 0 {
				names += ", "
			}
			names += name
			if i >= 4 {
				names += fmt.Sprintf(" (+%d more)", len(similar)-5)
				break
			}
		}
		findings = append(findings, DiagnosticFinding{
			Category: "labels",
			Summary:  fmt.Sprintf("Available %ss in namespace: %s", targetKind, names),
			Details:  map[string]string{"count": fmt.Sprintf("%d", len(similar))},
		})
	}

	return findings, nil
}

func refCodeToKind(code string) string {
	switch code {
	case "KOGARO-REF-001":
		return "IngressClass"
	case "KOGARO-REF-002":
		return "Service"
	case "KOGARO-REF-003":
		return "Secret"
	case "KOGARO-REF-004", "KOGARO-REF-005":
		return "ConfigMap"
	case "KOGARO-REF-006", "KOGARO-REF-007", "KOGARO-REF-008":
		return "Secret"
	case "KOGARO-REF-009":
		return "PersistentVolumeClaim"
	case "KOGARO-REF-010":
		return "StorageClass"
	case "KOGARO-REF-011":
		return "ServiceAccount"
	default:
		return "Resource"
	}
}

func resourceExists(ctx context.Context, c client.Client, namespace, name, kind string) (bool, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	var obj client.Object
	switch kind {
	case "ConfigMap":
		obj = &corev1.ConfigMap{}
	case "Secret":
		obj = &corev1.Secret{}
	case "Service":
		obj = &corev1.Service{}
	case "ServiceAccount":
		obj = &corev1.ServiceAccount{}
	case "PersistentVolumeClaim":
		obj = &corev1.PersistentVolumeClaim{}
	default:
		return false, fmt.Errorf("unsupported kind: %s", kind)
	}
	err := c.Get(ctx, key, obj)
	if err != nil {
		return false, nil
	}
	return true, nil
}

func listSimilarResources(ctx context.Context, c client.Client, namespace, kind string) []string {
	var names []string
	switch kind {
	case "ConfigMap":
		list := &corev1.ConfigMapList{}
		if err := c.List(ctx, list, client.InNamespace(namespace)); err == nil {
			for _, item := range list.Items {
				names = append(names, item.Name)
			}
		}
	case "Secret":
		list := &corev1.SecretList{}
		if err := c.List(ctx, list, client.InNamespace(namespace)); err == nil {
			for _, item := range list.Items {
				names = append(names, item.Name)
			}
		}
	case "Service":
		list := &corev1.ServiceList{}
		if err := c.List(ctx, list, client.InNamespace(namespace)); err == nil {
			for _, item := range list.Items {
				names = append(names, item.Name)
			}
		}
	}
	return names
}
