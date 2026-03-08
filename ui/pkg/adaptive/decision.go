package adaptive

import (
	"fmt"
	"strings"
)

// DecisionResult is the output of running an error code through a decision tree.
type DecisionResult struct {
	TreePath   string     `json:"treePath"`   // e.g. "SEC-002/known-image/uid-1001"
	Options    []FixOption `json:"options"`
	Warnings   []string   `json:"warnings,omitempty"`
	KBInsights []string   `json:"kbInsights,omitempty"`
}

// Decide runs the appropriate decision tree for an error code against a workload profile.
// Returns nil if no decision tree exists for the error code.
func Decide(errorCode string, profile *WorkloadProfile, containerName string) *DecisionResult {
	if profile == nil {
		return nil
	}

	// Find the target container
	cp := findContainer(profile, containerName)
	if cp == nil {
		return nil
	}

	switch errorCode {
	case "KOGARO-SEC-002":
		return decideSEC002(profile, cp)
	case "KOGARO-SEC-003":
		return decideSEC003(profile, cp)
	case "KOGARO-SEC-010":
		return decideSEC010(profile, cp)
	case "KOGARO-SEC-005":
		return decideSEC005(profile, cp)
	case "KOGARO-SEC-006":
		return decideSEC006(profile, cp)
	case "KOGARO-RES-002", "KOGARO-RES-005":
		return decideRES(errorCode, profile, cp)
	case "KOGARO-RES-UNKNOWN":
		return decideRESQoS(profile, cp)
	default:
		return nil
	}
}

// findContainer locates a container by name in the profile.
func findContainer(profile *WorkloadProfile, name string) *ContainerProfile {
	for i := range profile.Containers {
		if profile.Containers[i].Name == name {
			return &profile.Containers[i]
		}
	}
	// Fallback: return first container
	if len(profile.Containers) > 0 {
		return &profile.Containers[0]
	}
	return nil
}

// decideSEC002 handles "pod allows root" — runAsNonRoot is not set or false.
func decideSEC002(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	if cp.IsKnownImage && cp.KnownTraits != nil {
		uid := cp.KnownTraits.DefaultUID
		r.TreePath = fmt.Sprintf("SEC-002/known-image/uid-%d", uid)
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Image %s is known to run as UID %d", cp.ImageBase, uid),
		)
		if cp.KnownTraits.Notes != "" {
			r.KBInsights = append(r.KBInsights, cp.KnownTraits.Notes)
		}

		// Option 1: Use the known UID
		r.Options = append(r.Options, FixOption{
			Label:       fmt.Sprintf("Set runAsNonRoot with UID %d (known safe for %s)", uid, cp.ImageBase),
			Description: fmt.Sprintf("Uses the default UID (%d) for %s. This is the safest approach because the image is designed to run as this user.", uid, cp.ImageBase),
			Risk:        "low",
			Commands: []FixCmd{{
				Label:   fmt.Sprintf("Set pod security context (UID %d)", uid),
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":%d}}}}}'`, kind, name, ns, uid),
			}},
			Rollback: []FixCmd{{
				Label:   "Remove runAsNonRoot and runAsUser",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/securityContext/runAsNonRoot"},{"op":"remove","path":"/spec/template/spec/securityContext/runAsUser"}]'`, kind, name, ns),
			}},
		})

		// Option 2: Generic UID 65534 (nobody) if different from known
		if uid != 65534 {
			r.Options = append(r.Options, FixOption{
				Label:       "Set runAsNonRoot with UID 65534 (nobody)",
				Description: "Uses the 'nobody' user (65534). Works universally but the image may not have correct file permissions for this UID.",
				Risk:        "medium",
				Warnings: []string{
					fmt.Sprintf("Image %s normally runs as UID %d — UID 65534 may cause permission errors", cp.ImageBase, uid),
				},
				Commands: []FixCmd{{
					Label:   "Set pod security context (UID 65534)",
					Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":65534}}}}}'`, kind, name, ns),
				}},
				Rollback: []FixCmd{{
					Label:   "Remove runAsNonRoot and runAsUser",
					Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/securityContext/runAsNonRoot"},{"op":"remove","path":"/spec/template/spec/securityContext/runAsUser"}]'`, kind, name, ns),
				}},
			})
		}
	} else {
		// Unknown image — can't be sure about UID
		r.TreePath = "SEC-002/unknown-image"
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("Image %s is not in the knowledge base — check the Dockerfile for the USER directive before applying", cp.Image),
		)
		r.KBInsights = append(r.KBInsights,
			"Unknown image: verify the expected UID by checking the Dockerfile or running the image locally",
		)

		// Option 1: Conservative — runAsNonRoot with 65534
		r.Options = append(r.Options, FixOption{
			Label:       "Set runAsNonRoot with UID 65534 (nobody) — verify image compatibility first",
			Description: "Uses the 'nobody' user. Safe if the image doesn't need specific file ownership. Check the image's Dockerfile USER directive first.",
			Risk:        "medium",
			Warnings:    []string{"Unknown image — may fail if the image requires a specific UID for file permissions"},
			Commands: []FixCmd{
				{
					Label:   "Check current image",
					Command: fmt.Sprintf("kubectl get %s %s -n %s -o jsonpath='{.spec.template.spec.containers[0].image}'", kind, name, ns),
				},
				{
					Label:   "Set pod security context (UID 65534)",
					Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":65534}}}}}'`, kind, name, ns),
				},
			},
			Rollback: []FixCmd{{
				Label:   "Remove runAsNonRoot and runAsUser",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/securityContext/runAsNonRoot"},{"op":"remove","path":"/spec/template/spec/securityContext/runAsUser"}]'`, kind, name, ns),
			}},
		})
	}

	// If already has runAsUser at container level, note it
	if cp.HasSecurityCtx && cp.SecurityCtx != nil && cp.SecurityCtx.RunAsUser != nil {
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Container already has runAsUser=%d at container level", *cp.SecurityCtx.RunAsUser),
		)
	}

	return r
}

// decideSEC003 handles "privilege escalation allowed".
func decideSEC003(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{TreePath: "SEC-003/deny-escalation"}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	r.Options = append(r.Options, FixOption{
		Label:       "Deny privilege escalation",
		Description: "Sets allowPrivilegeEscalation=false. This is safe for nearly all workloads — only sudo/setuid binaries require it.",
		Risk:        "low",
		Commands: []FixCmd{{
			Label:       fmt.Sprintf("Set allowPrivilegeEscalation=false for %s", cp.Name),
			Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"allowPrivilegeEscalation":false}}]}}}}'`, kind, name, ns, cp.Name),
			Destructive: true,
		}},
		Rollback: []FixCmd{{
			Label:   "Remove allowPrivilegeEscalation setting",
			Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext/allowPrivilegeEscalation"}]'`, kind, name, ns),
		}},
	})

	if cp.NeedsPrivPort {
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("Container %s binds to a privileged port (<1024) — may need NET_BIND_SERVICE capability", cp.Name),
		)
	}

	return r
}

// decideSEC005 handles "privileged container".
func decideSEC005(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{TreePath: "SEC-005/remove-privileged"}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	r.Warnings = append(r.Warnings,
		"Privileged containers have full host access — this is almost never needed",
	)

	r.Options = append(r.Options, FixOption{
		Label:       "Remove privileged mode",
		Description: "Sets privileged=false. Most workloads never need privileged mode. Only CNI plugins, storage drivers, and some monitoring agents require it.",
		Risk:        "medium",
		Warnings:    []string{"Verify the workload doesn't need direct host device access before applying"},
		Commands: []FixCmd{{
			Label:       fmt.Sprintf("Set privileged=false for %s", cp.Name),
			Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"privileged":false}}]}}}}'`, kind, name, ns, cp.Name),
			Destructive: true,
		}},
		Rollback: []FixCmd{{
			Label:   "Restore privileged mode",
			Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"privileged":true}}]}}}}'`, kind, name, ns, cp.Name),
		}},
	})

	return r
}

// decideSEC006 handles "writable root filesystem" (readOnlyRootFilesystem not set or false).
func decideSEC006(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	if cp.IsKnownImage && cp.KnownTraits != nil {
		if cp.KnownTraits.SafeForReadOnlyFS {
			r.TreePath = "SEC-006/known-image/safe-readonly"
			r.KBInsights = append(r.KBInsights,
				fmt.Sprintf("Image %s is safe for readOnlyRootFilesystem", cp.ImageBase),
			)
			r.Options = append(r.Options, FixOption{
				Label:       "Enable readOnlyRootFilesystem (safe for this image)",
				Description: fmt.Sprintf("Image %s doesn't need writable root filesystem. This hardens the container against runtime file tampering.", cp.ImageBase),
				Risk:        "low",
				Commands: []FixCmd{{
					Label:       fmt.Sprintf("Set readOnlyRootFilesystem=true for %s", cp.Name),
					Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"readOnlyRootFilesystem":true}}]}}}}'`, kind, name, ns, cp.Name),
					Destructive: true,
				}},
				Rollback: []FixCmd{{
					Label:   "Remove readOnlyRootFilesystem",
					Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext/readOnlyRootFilesystem"}]'`, kind, name, ns),
				}},
			})
		} else {
			r.TreePath = "SEC-006/known-image/needs-writable"
			r.KBInsights = append(r.KBInsights,
				fmt.Sprintf("Image %s needs writable paths: %s", cp.ImageBase, strings.Join(cp.KnownTraits.WritablePaths, ", ")),
			)
			r.Warnings = append(r.Warnings,
				fmt.Sprintf("Do NOT enable readOnlyRootFilesystem without adding emptyDir volumes for: %s", strings.Join(cp.KnownTraits.WritablePaths, ", ")),
			)

			// Option 1: readOnlyRootFilesystem + emptyDir volumes
			var volumePatches []string
			var mountPatches []string
			for i, path := range cp.KnownTraits.WritablePaths {
				volName := fmt.Sprintf("writable-%d", i)
				volumePatches = append(volumePatches, fmt.Sprintf(`{"name":"%s","emptyDir":{}}`, volName))
				mountPatches = append(mountPatches, fmt.Sprintf(`{"name":"%s","mountPath":"%s"}`, volName, path))
			}

			r.Options = append(r.Options, FixOption{
				Label:       fmt.Sprintf("Enable readOnlyRootFilesystem with emptyDir volumes for writable paths"),
				Description: fmt.Sprintf("Adds emptyDir volumes for %s's known writable paths (%s), then enables readOnlyRootFilesystem.", cp.ImageBase, strings.Join(cp.KnownTraits.WritablePaths, ", ")),
				Risk:        "medium",
				Commands: []FixCmd{{
					Label:       "Add emptyDir volumes and enable readOnlyRootFilesystem",
					Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"volumes":[%s],"containers":[{"name":"%s","securityContext":{"readOnlyRootFilesystem":true},"volumeMounts":[%s]}]}}}}'`, kind, name, ns, strings.Join(volumePatches, ","), cp.Name, strings.Join(mountPatches, ",")),
					Destructive: true,
				}},
				Rollback: []FixCmd{{
					Label:   "Remove readOnlyRootFilesystem",
					Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext/readOnlyRootFilesystem"}]'`, kind, name, ns),
				}},
			})

			// Option 2: Skip — acknowledge the risk
			r.Options = append(r.Options, FixOption{
				Label:       "Skip readOnlyRootFilesystem (accept the risk)",
				Description: fmt.Sprintf("Image %s needs writable paths. Skipping readOnlyRootFilesystem avoids the complexity of emptyDir volume mounts.", cp.ImageBase),
				Risk:        "low",
				Warnings:    []string{"Container root filesystem will remain writable — this is a security trade-off"},
				Commands:    nil, // No action needed
			})
		}
	} else {
		r.TreePath = "SEC-006/unknown-image"
		r.Warnings = append(r.Warnings,
			"Unknown image — readOnlyRootFilesystem may break the application if it writes to disk at runtime",
		)
		r.KBInsights = append(r.KBInsights,
			"Test with readOnlyRootFilesystem in a non-production environment first",
		)

		r.Options = append(r.Options, FixOption{
			Label:       "Enable readOnlyRootFilesystem (test first)",
			Description: "May break the application if it writes to the filesystem at runtime. Test in a non-production environment first.",
			Risk:        "high",
			Warnings:    []string{"Unknown image — may crash if the application writes to disk"},
			Commands: []FixCmd{{
				Label:       fmt.Sprintf("Set readOnlyRootFilesystem=true for %s", cp.Name),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"readOnlyRootFilesystem":true}}]}}}}'`, kind, name, ns, cp.Name),
				Destructive: true,
			}},
			Rollback: []FixCmd{{
				Label:   "Remove readOnlyRootFilesystem",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext/readOnlyRootFilesystem"}]'`, kind, name, ns),
			}},
		})

		r.Options = append(r.Options, FixOption{
			Label:       "Skip readOnlyRootFilesystem",
			Description: "Accept the risk of a writable root filesystem. Safer for unknown images.",
			Risk:        "low",
			Commands:    nil,
		})
	}

	return r
}

// decideSEC010 handles "missing container security context".
func decideSEC010(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	if cp.IsKnownImage && cp.KnownTraits != nil {
		r.TreePath = fmt.Sprintf("SEC-010/known-image/%s", cp.ImageBase)
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Image %s: UID %d, writable paths: %s", cp.ImageBase, cp.KnownTraits.DefaultUID, strings.Join(cp.KnownTraits.WritablePaths, ", ")),
		)

		// Build a tailored security context
		patchParts := []string{
			`"allowPrivilegeEscalation":false`,
			`"capabilities":{"drop":["ALL"]}`,
		}
		desc := "Hardened defaults: drop all capabilities, deny privilege escalation"

		if cp.KnownTraits.SafeForReadOnlyFS {
			patchParts = append(patchParts, `"readOnlyRootFilesystem":true`)
			desc += ", read-only root filesystem"
		}

		// Add any required capabilities
		if len(cp.KnownTraits.NeedsCapabilities) > 0 {
			caps := make([]string, len(cp.KnownTraits.NeedsCapabilities))
			for i, c := range cp.KnownTraits.NeedsCapabilities {
				caps[i] = fmt.Sprintf(`"%s"`, c)
			}
			patchParts = append(patchParts, fmt.Sprintf(`"capabilities":{"drop":["ALL"],"add":[%s]}`, strings.Join(caps, ",")))
			// Remove the earlier drop-only capabilities
			filtered := make([]string, 0, len(patchParts))
			seenCaps := false
			for _, p := range patchParts {
				if strings.HasPrefix(p, `"capabilities"`) {
					if !seenCaps {
						seenCaps = true
						continue // skip first (drop-only)
					}
				}
				filtered = append(filtered, p)
			}
			patchParts = filtered
		}

		secCtx := strings.Join(patchParts, ",")

		r.Options = append(r.Options, FixOption{
			Label:       fmt.Sprintf("Add tailored security context for %s (%s)", cp.Name, cp.ImageBase),
			Description: desc,
			Risk:        "low",
			Commands: []FixCmd{{
				Label:       fmt.Sprintf("Add security context for %s", cp.Name),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{%s}}]}}}}'`, kind, name, ns, cp.Name, secCtx),
				Destructive: true,
			}},
			Rollback: []FixCmd{{
				Label:   "Remove container security context",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext"}]'`, kind, name, ns),
			}},
		})
	} else {
		r.TreePath = "SEC-010/unknown-image"
		r.Warnings = append(r.Warnings,
			fmt.Sprintf("Image %s is not in the knowledge base — using conservative defaults (no readOnlyRootFilesystem)", cp.Image),
		)

		r.Options = append(r.Options, FixOption{
			Label:       fmt.Sprintf("Add conservative security context for %s", cp.Name),
			Description: "Safe defaults: drop all capabilities, deny privilege escalation. Does NOT enable readOnlyRootFilesystem (unknown image may need writable disk).",
			Risk:        "low",
			Commands: []FixCmd{{
				Label:       fmt.Sprintf("Add security context for %s", cp.Name),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","securityContext":{"allowPrivilegeEscalation":false,"capabilities":{"drop":["ALL"]}}}]}}}}'`, kind, name, ns, cp.Name),
				Destructive: true,
			}},
			Rollback: []FixCmd{{
				Label:   "Remove container security context",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/securityContext"}]'`, kind, name, ns),
			}},
		})
	}

	return r
}

// decideRES handles missing resource requests/limits (RES-002, RES-005).
func decideRES(errorCode string, profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{TreePath: fmt.Sprintf("%s/resources", strings.TrimPrefix(errorCode, "KOGARO-"))}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	// Check existing resources
	if cp.Resources != nil {
		r.KBInsights = append(r.KBInsights,
			fmt.Sprintf("Current resources: requests(cpu=%s, mem=%s) limits(cpu=%s, mem=%s)",
				orDefault(cp.Resources.CPURequest, "unset"),
				orDefault(cp.Resources.MemoryRequest, "unset"),
				orDefault(cp.Resources.CPULimit, "unset"),
				orDefault(cp.Resources.MemoryLimit, "unset"),
			),
		)
	}

	// Option 1: Conservative defaults
	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Set conservative resource defaults for %s", cp.Name),
		Description: "Sets requests=50m/64Mi, limits=200m/256Mi. Suitable for small services and sidecars.",
		Risk:        "low",
		Commands: []FixCmd{{
			Label:       fmt.Sprintf("Set resource requests and limits for %s", cp.Name),
			Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"cpu":"50m","memory":"64Mi"},"limits":{"cpu":"200m","memory":"256Mi"}}}]}}}}'`, kind, name, ns, cp.Name),
			Destructive: true,
		}},
		Rollback: []FixCmd{{
			Label:   "Remove resource constraints",
			Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/resources"}]'`, kind, name, ns),
		}},
	})

	// Option 2: Generous defaults for databases/stateful workloads
	if cp.IsKnownImage && cp.KnownTraits != nil && !cp.KnownTraits.SafeForReadOnlyFS {
		// Likely a database or stateful application
		r.Options = append(r.Options, FixOption{
			Label:       fmt.Sprintf("Set generous resource defaults for %s (data-intensive workload)", cp.Name),
			Description: "Sets requests=250m/256Mi, limits=1000m/1Gi. Suitable for databases and data-intensive workloads.",
			Risk:        "low",
			Commands: []FixCmd{{
				Label:       fmt.Sprintf("Set resource requests and limits for %s", cp.Name),
				Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"cpu":"250m","memory":"256Mi"},"limits":{"cpu":"1000m","memory":"1Gi"}}}]}}}}'`, kind, name, ns, cp.Name),
				Destructive: true,
			}},
			Rollback: []FixCmd{{
				Label:   "Remove resource constraints",
				Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/resources"}]'`, kind, name, ns),
			}},
		})
	}

	return r
}

// decideRESQoS handles QoS class issues (Burstable → Guaranteed).
func decideRESQoS(profile *WorkloadProfile, cp *ContainerProfile) *DecisionResult {
	r := &DecisionResult{TreePath: "RES-QoS"}
	kind := strings.ToLower(profile.Kind)
	ns := profile.Namespace
	name := profile.Name

	// Read current limits from the profile
	cpuLimit := "200m"
	memLimit := "256Mi"
	if cp.Resources != nil {
		if cp.Resources.CPULimit != "" {
			cpuLimit = cp.Resources.CPULimit
		}
		if cp.Resources.MemoryLimit != "" {
			memLimit = cp.Resources.MemoryLimit
		}
	}

	r.KBInsights = append(r.KBInsights,
		fmt.Sprintf("Current limits: cpu=%s, memory=%s", cpuLimit, memLimit),
	)

	// Option 1: Set requests = limits (Guaranteed QoS)
	r.Options = append(r.Options, FixOption{
		Label:       fmt.Sprintf("Set Guaranteed QoS for %s (requests = limits: %s/%s)", cp.Name, cpuLimit, memLimit),
		Description: "Sets resource requests equal to limits, achieving Guaranteed QoS class. Pods get priority scheduling and are last to be evicted.",
		Risk:        "low",
		Commands: []FixCmd{{
			Label:       fmt.Sprintf("Set Guaranteed QoS for %s", cp.Name),
			Command:     fmt.Sprintf(`kubectl patch %s %s -n %s --type=strategic -p '{"spec":{"template":{"spec":{"containers":[{"name":"%s","resources":{"requests":{"memory":"%s","cpu":"%s"},"limits":{"memory":"%s","cpu":"%s"}}}]}}}}'`, kind, name, ns, cp.Name, memLimit, cpuLimit, memLimit, cpuLimit),
			Destructive: true,
		}},
		Rollback: []FixCmd{{
			Label:   "Remove resource requests (back to Burstable)",
			Command: fmt.Sprintf(`kubectl patch %s %s -n %s --type=json -p '[{"op":"remove","path":"/spec/template/spec/containers/0/resources/requests"}]'`, kind, name, ns),
		}},
	})

	return r
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
