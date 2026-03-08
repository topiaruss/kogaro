package adaptive

import (
	"fmt"
	"strings"
)

// NodeContext provides node-level information for decision trees that don't
// operate on container profiles (e.g., NET and REF error codes).
type NodeContext struct {
	Kind         string            // "Service", "Ingress", "ConfigMap", etc.
	Name         string            // resource name
	Namespace    string            // resource namespace
	ErrorCode    string            // the KOGARO error code
	OwnerKind    string            // parent workload kind (e.g., "Deployment")
	OwnerName    string            // parent workload name
	Selector     string            // label selector (for services)
	TargetName   string            // name of the missing/broken target resource
	TargetKind   string            // kind of the missing/broken target resource
	Details      map[string]string // additional context from diagnostics
}

// DecideForNode runs decision trees for error codes that operate on
// non-workload resources (Services, Ingresses, dangling references).
// Returns nil if no decision tree exists for the error code.
func DecideForNode(nc *NodeContext) *DecisionResult {
	if nc == nil {
		return nil
	}

	switch nc.ErrorCode {
	// Networking
	case "KOGARO-NET-001":
		return decideNET001(nc)
	case "KOGARO-NET-002":
		return decideNET002(nc)
	case "KOGARO-NET-003":
		return decideNET003(nc)
	case "KOGARO-NET-005":
		return decideNET005(nc)
	case "KOGARO-NET-007":
		return decideNET007(nc)
	case "KOGARO-NET-008":
		return decideNET008(nc)
	case "KOGARO-NET-009":
		return decideNET009(nc)

	// Dangling references
	case "KOGARO-REF-001":
		return decideREF001(nc)
	case "KOGARO-REF-002":
		return decideREFMissing(nc, "Service")
	case "KOGARO-REF-003":
		return decideREFSecret(nc, "tls")
	case "KOGARO-REF-004", "KOGARO-REF-005":
		return decideREFConfigMap(nc)
	case "KOGARO-REF-006", "KOGARO-REF-007", "KOGARO-REF-008":
		return decideREFSecret(nc, "")
	case "KOGARO-REF-009":
		return decideREFStorage(nc, "StorageClass")
	case "KOGARO-REF-010":
		return decideREFStorage(nc, "PersistentVolumeClaim")
	case "KOGARO-REF-011":
		return decideREFServiceAccount(nc)

	default:
		return nil
	}
}

// --- Networking decision trees ---

// decideNET001: service selector doesn't match any pods.
func decideNET001(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-001/selector-mismatch"}
	ns := nc.Namespace
	name := nc.Name

	r.KBInsights = append(r.KBInsights,
		"Service selector doesn't match any pod labels in the namespace",
	)
	if nc.Selector != "" {
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Current selector: %s", nc.Selector),
		)
	}

	// Option 1: Fix the selector to match pods
	r.Options = append(r.Options, FixOption{
		Label:       "Investigate and fix the selector",
		Description: "Compare the service selector with actual pod labels. The selector may have a typo or the deployment labels may have changed.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show service selector", Command: fmt.Sprintf("kubectl get svc %s -n %s -o jsonpath='{.spec.selector}'", name, ns)},
			{Label: "List pods with labels", Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns)},
		},
	})

	// Option 2: If we know the selector, offer a patch
	if nc.Selector != "" {
		r.Options = append(r.Options, FixOption{
			Label:       "Check if pods exist with different labels",
			Description: "If pods exist but labels don't match, either patch the service selector or the deployment's pod template labels.",
			Risk:        "medium",
			Commands: []FixCmd{
				{Label: "Find pods matching selector", Command: fmt.Sprintf("kubectl get pods -n %s -l %s", ns, nc.Selector)},
				{Label: "Show all deployments", Command: fmt.Sprintf("kubectl get deployments -n %s -o wide", ns)},
			},
		})
	}

	return r
}

// decideNET002: service has no endpoints.
func decideNET002(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-002/no-endpoints"}
	ns := nc.Namespace
	name := nc.Name

	r.KBInsights = append(r.KBInsights,
		"Service has no ready endpoints — pods may not exist, not be ready, or labels may not match",
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Diagnose missing endpoints",
		Description: "Check if pods exist and are ready. If pods exist but aren't endpoints, the readiness probe may be failing.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Check endpoint slices", Command: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", ns, name)},
			{Label: "Show service details", Command: fmt.Sprintf("kubectl describe svc %s -n %s", name, ns)},
		},
	})

	if nc.Selector != "" {
		r.Options = append(r.Options, FixOption{
			Label:       "Check pods matching service selector",
			Description: fmt.Sprintf("Find pods with selector %s and verify they're Ready.", nc.Selector),
			Risk:        "low",
			Commands: []FixCmd{
				{Label: "Find matching pods", Command: fmt.Sprintf("kubectl get pods -n %s -l %s -o wide", ns, nc.Selector)},
				{Label: "Check pod readiness", Command: fmt.Sprintf("kubectl get pods -n %s -l %s -o jsonpath='{range .items[*]}{.metadata.name}{\"\\t\"}{.status.phase}{\"\\t\"}{range .status.conditions[?(@.type==\"Ready\")]}{.status}{end}{\"\\n\"}{end}'", ns, nc.Selector)},
			},
		})
	}

	return r
}

// decideNET003: service port doesn't match container port.
func decideNET003(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-003/port-mismatch"}
	ns := nc.Namespace
	name := nc.Name

	r.KBInsights = append(r.KBInsights,
		"Service targetPort doesn't match any container port in the backing pods",
	)
	r.Warnings = append(r.Warnings,
		"Traffic will not reach the container — fix the targetPort or container port",
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Compare service ports with container ports",
		Description: "Show the service's targetPort and the pods' containerPort values side by side to identify the mismatch.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show service ports", Command: fmt.Sprintf("kubectl get svc %s -n %s -o jsonpath='{.spec.ports}'", name, ns)},
			{Label: "Show pod container ports", Command: fmt.Sprintf("kubectl get pods -n %s -l %s -o jsonpath='{range .items[0].spec.containers[*]}{.name}: {.ports}{\"\\n\"}{end}'", ns, nc.Selector)},
		},
	})

	return r
}

// decideNET005: NetworkPolicy selector matches no pods.
func decideNET005(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-005/orphaned-policy"}
	ns := nc.Namespace
	name := nc.Name

	r.KBInsights = append(r.KBInsights,
		"NetworkPolicy's podSelector doesn't match any pods — the policy has no effect",
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Investigate orphaned NetworkPolicy",
		Description: "Check if the target pods were deleted or their labels changed. The policy may be stale.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show NetworkPolicy selector", Command: fmt.Sprintf("kubectl get networkpolicy %s -n %s -o jsonpath='{.spec.podSelector}'", name, ns)},
			{Label: "List pods with labels", Command: fmt.Sprintf("kubectl get pods -n %s --show-labels", ns)},
		},
	})

	r.Options = append(r.Options, FixOption{
		Label:       "Delete orphaned NetworkPolicy",
		Description: "If the target workload no longer exists, remove the stale policy.",
		Risk:        "medium",
		Warnings:    []string{"Verify the target pods are truly gone before deleting"},
		Commands: []FixCmd{
			{Label: fmt.Sprintf("Delete NetworkPolicy %s", name), Command: fmt.Sprintf("kubectl delete networkpolicy %s -n %s", name, ns), Destructive: true},
		},
		Rollback: []FixCmd{
			{Label: "Re-export the policy YAML (if backed up)", Command: fmt.Sprintf("kubectl get networkpolicy %s -n %s -o yaml", name, ns)},
		},
	})

	return r
}

// decideNET007: Ingress references a missing Service.
func decideNET007(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-007/ingress-missing-svc"}
	ns := nc.Namespace
	name := nc.Name

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("Ingress %s references Service '%s' which does not exist", name, target),
	)
	r.Warnings = append(r.Warnings,
		fmt.Sprintf("Ingress traffic for routes pointing to '%s' will return 503", target),
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Create the missing Service or fix the Ingress",
		Description: fmt.Sprintf("Either create Service '%s' or update the Ingress backend to point to an existing Service.", target),
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show Ingress backends", Command: fmt.Sprintf("kubectl get ingress %s -n %s -o yaml", name, ns)},
			{Label: "List available Services", Command: fmt.Sprintf("kubectl get svc -n %s", ns)},
		},
	})

	return r
}

// decideNET008: Ingress references a port that doesn't exist on the Service.
func decideNET008(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-008/ingress-port-mismatch"}
	ns := nc.Namespace
	name := nc.Name

	r.KBInsights = append(r.KBInsights,
		"Ingress references a port that does not exist on the target Service",
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Compare Ingress port with Service ports",
		Description: "Show the Ingress backend port and the Service's available ports to identify the mismatch.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show Ingress spec", Command: fmt.Sprintf("kubectl get ingress %s -n %s -o jsonpath='{.spec.rules}'", name, ns)},
			{Label: "Show Service ports", Command: fmt.Sprintf("kubectl get svc -n %s -o custom-columns=NAME:.metadata.name,PORTS:.spec.ports[*].port", ns)},
		},
	})

	return r
}

// decideNET009: Ingress Service has no ready backend pods.
func decideNET009(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "NET-009/ingress-no-backends"}
	ns := nc.Namespace
	name := nc.Name

	target := nc.TargetName
	if target == "" {
		target = name
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("Ingress routes to Service '%s' which has no ready backends — this is usually a symptom", target),
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Trace from Ingress to pods",
		Description: "Follow the chain: Ingress → Service → Pods to find where it breaks.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "Show Ingress backends", Command: fmt.Sprintf("kubectl get ingress %s -n %s", name, ns)},
			{Label: fmt.Sprintf("Describe Service %s", target), Command: fmt.Sprintf("kubectl describe svc %s -n %s", target, ns)},
			{Label: fmt.Sprintf("Check endpoints for %s", target), Command: fmt.Sprintf("kubectl get endpointslices -n %s -l kubernetes.io/service-name=%s", ns, target)},
		},
	})

	return r
}

// --- Reference decision trees ---

// decideREF001: missing IngressClass.
func decideREF001(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "REF-001/missing-ingressclass"}
	ns := nc.Namespace
	name := nc.Name

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("Ingress %s references IngressClass '%s' which does not exist", name, target),
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Check available IngressClasses",
		Description: "IngressClass is cluster-scoped. List available ones and update the Ingress to use an existing class.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "List IngressClasses", Command: "kubectl get ingressclasses"},
			{Label: "Show Ingress ingressClassName", Command: fmt.Sprintf("kubectl get ingress %s -n %s -o jsonpath='{.spec.ingressClassName}'", name, ns)},
		},
	})

	return r
}

// decideREFMissing: generic missing service reference from Ingress.
func decideREFMissing(nc *NodeContext, targetKind string) *DecisionResult {
	code := strings.TrimPrefix(nc.ErrorCode, "KOGARO-")
	r := &DecisionResult{TreePath: fmt.Sprintf("%s/missing-%s", code, strings.ToLower(targetKind))}
	ns := nc.Namespace

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("%s references %s '%s' which does not exist", nc.Name, targetKind, target),
	)

	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Create missing %s or fix the reference", targetKind),
		Description: fmt.Sprintf("Either create %s '%s' or update the reference to point to an existing resource.", targetKind, target),
		Risk:        "low",
		Commands: []FixCmd{
			{Label: fmt.Sprintf("List %ss in namespace", targetKind), Command: fmt.Sprintf("kubectl get %s -n %s", strings.ToLower(targetKind), ns)},
			{Label: fmt.Sprintf("Describe source %s", nc.Kind), Command: fmt.Sprintf("kubectl describe %s %s -n %s", strings.ToLower(nc.Kind), nc.Name, ns)},
		},
	})

	return r
}

// decideREFConfigMap: missing ConfigMap (volume or envFrom).
func decideREFConfigMap(nc *NodeContext) *DecisionResult {
	code := strings.TrimPrefix(nc.ErrorCode, "KOGARO-")
	usage := "volume"
	if nc.ErrorCode == "KOGARO-REF-005" {
		usage = "envFrom"
	}
	r := &DecisionResult{TreePath: fmt.Sprintf("%s/missing-configmap/%s", code, usage)}
	ns := nc.Namespace

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("ConfigMap '%s' used as %s does not exist — pods referencing it will fail to start", target, usage),
	)
	r.Warnings = append(r.Warnings,
		"Pods will be stuck in ContainerCreating until this ConfigMap exists",
	)

	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Create the missing ConfigMap '%s'", target),
		Description: "Create an empty ConfigMap as a placeholder, then populate it with the correct data.",
		Risk:        "medium",
		Warnings:    []string{"An empty ConfigMap may not have the keys the pod expects — add the required keys"},
		Commands: []FixCmd{
			{Label: "List existing ConfigMaps", Command: fmt.Sprintf("kubectl get configmaps -n %s", ns)},
			{Label: fmt.Sprintf("Create empty ConfigMap %s", target), Command: fmt.Sprintf("kubectl create configmap %s -n %s", target, ns), Destructive: true},
		},
		Rollback: []FixCmd{
			{Label: fmt.Sprintf("Delete ConfigMap %s", target), Command: fmt.Sprintf("kubectl delete configmap %s -n %s", target, ns)},
		},
	})

	// Option 2: Make the reference optional
	if usage == "volume" && nc.OwnerKind != "" {
		r.Options = append(r.Options, FixOption{
			Label:       "Make the ConfigMap volume optional",
			Description: "Set optional=true on the volume source so the pod starts even if the ConfigMap doesn't exist.",
			Risk:        "medium",
			Warnings:    []string{"The application may fail at runtime if it expects the ConfigMap data"},
			Commands: []FixCmd{
				{Label: "Show volume spec", Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.volumes}'", strings.ToLower(nc.OwnerKind), nc.OwnerName, ns)},
			},
		})
	}

	return r
}

// decideREFSecret: missing Secret (volume, envFrom, env, or TLS).
func decideREFSecret(nc *NodeContext, usage string) *DecisionResult {
	code := strings.TrimPrefix(nc.ErrorCode, "KOGARO-")
	if usage == "" {
		switch nc.ErrorCode {
		case "KOGARO-REF-006":
			usage = "volume"
		case "KOGARO-REF-007":
			usage = "envFrom"
		case "KOGARO-REF-008":
			usage = "env"
		}
	}
	r := &DecisionResult{TreePath: fmt.Sprintf("%s/missing-secret/%s", code, usage)}
	ns := nc.Namespace

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	if usage == "tls" {
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("TLS Secret '%s' does not exist — HTTPS will not work for this Ingress", target),
		)
		r.Warnings = append(r.Warnings,
			"Ingress controller will serve with default certificate or fail TLS termination",
		)
	} else {
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Secret '%s' used as %s does not exist — pods referencing it will fail to start", target, usage),
		)
		r.Warnings = append(r.Warnings,
			"Pods will be stuck in ContainerCreating until this Secret exists",
		)
	}

	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Create the missing Secret '%s'", target),
		Description: "Create the Secret with the required data. For TLS secrets, use kubectl create secret tls.",
		Risk:        "medium",
		Commands: []FixCmd{
			{Label: "List existing Secrets", Command: fmt.Sprintf("kubectl get secrets -n %s", ns)},
			{Label: fmt.Sprintf("Describe source %s", nc.Kind), Command: fmt.Sprintf("kubectl describe %s %s -n %s", strings.ToLower(nc.Kind), nc.Name, ns)},
		},
	})

	if usage == "tls" {
		r.KBInsights = append(r.KBInsights,
			"If using cert-manager, check that the Certificate resource exists and is ready",
		)
		r.Options = append(r.Options, FixOption{
			Label:       "Check cert-manager Certificate status",
			Description: "If cert-manager manages this certificate, check its status and events.",
			Risk:        "low",
			Commands: []FixCmd{
				{Label: "List Certificates", Command: fmt.Sprintf("kubectl get certificates -n %s", ns)},
				{Label: "Check cert-manager events", Command: fmt.Sprintf("kubectl get events -n %s --sort-by=.lastTimestamp --field-selector reason=Issuing", ns)},
			},
		})
	}

	return r
}

// decideREFStorage: missing StorageClass or PVC.
func decideREFStorage(nc *NodeContext, targetKind string) *DecisionResult {
	code := strings.TrimPrefix(nc.ErrorCode, "KOGARO-")
	r := &DecisionResult{TreePath: fmt.Sprintf("%s/missing-%s", code, strings.ToLower(targetKind))}
	ns := nc.Namespace

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("%s '%s' does not exist", targetKind, target),
	)

	if targetKind == "StorageClass" {
		r.Options = append(r.Options, FixOption{
			Label:       "Check available StorageClasses",
			Description: "StorageClass is cluster-scoped. List available ones and update the PVC to use an existing class, or install the required storage provisioner.",
			Risk:        "low",
			Commands: []FixCmd{
				{Label: "List StorageClasses", Command: "kubectl get storageclasses"},
				{Label: "Show PVC storageClassName", Command: fmt.Sprintf("kubectl get pvc %s -n %s -o jsonpath='{.spec.storageClassName}'", nc.Name, ns)},
			},
		})
	} else {
		// PVC
		r.Warnings = append(r.Warnings,
			"Pods referencing this PVC will be stuck in Pending until it exists",
		)
		r.Options = append(r.Options, FixOption{
			Label:       "Check if PVC was deleted or never created",
			Description: "List existing PVCs and check if the name was changed or the PVC needs to be recreated.",
			Risk:        "low",
			Commands: []FixCmd{
				{Label: "List PVCs", Command: fmt.Sprintf("kubectl get pvc -n %s", ns)},
				{Label: "Show volume references", Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.volumes}'", strings.ToLower(nc.Kind), nc.Name, ns)},
			},
		})
	}

	return r
}

// decideREFServiceAccount: missing ServiceAccount.
func decideREFServiceAccount(nc *NodeContext) *DecisionResult {
	r := &DecisionResult{TreePath: "REF-011/missing-serviceaccount"}
	ns := nc.Namespace

	target := nc.TargetName
	if target == "" {
		target = "<unknown>"
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("ServiceAccount '%s' does not exist — pods will fail to be created by the admission controller", target),
	)

	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Create ServiceAccount '%s'", target),
		Description: "Create the missing ServiceAccount. If RBAC bindings reference it, those will also need to be configured.",
		Risk:        "low",
		Commands: []FixCmd{
			{Label: "List ServiceAccounts", Command: fmt.Sprintf("kubectl get serviceaccounts -n %s", ns)},
			{Label: fmt.Sprintf("Create ServiceAccount %s", target), Command: fmt.Sprintf("kubectl create serviceaccount %s -n %s", target, ns), Destructive: true},
		},
		Rollback: []FixCmd{
			{Label: fmt.Sprintf("Delete ServiceAccount %s", target), Command: fmt.Sprintf("kubectl delete serviceaccount %s -n %s", target, ns)},
		},
	})

	r.Options = append(r.Options, FixOption{
		Label:       "Use default ServiceAccount instead",
		Description: "Patch the workload to use the namespace's default ServiceAccount.",
		Risk:        "medium",
		Warnings:    []string{"The workload may need specific RBAC permissions that the default account doesn't have"},
		Commands: []FixCmd{
			{Label: "Patch to use default SA", Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"serviceAccountName":"default"}}}}'`, strings.ToLower(nc.OwnerKind), nc.OwnerName, ns), Destructive: true},
		},
		Rollback: []FixCmd{
			{Label: "Restore original SA", Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"serviceAccountName":"%s"}}}}'`, strings.ToLower(nc.OwnerKind), nc.OwnerName, ns, target)},
		},
	})

	return r
}
