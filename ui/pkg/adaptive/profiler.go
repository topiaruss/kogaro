package adaptive

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Profiler builds WorkloadProfiles from live cluster workloads.
type Profiler struct {
	client client.Client
}

// NewProfiler creates a Profiler using the given controller-runtime client.
func NewProfiler(c client.Client) *Profiler {
	return &Profiler{client: c}
}

// ProfileWorkload fetches a workload from the cluster and builds a WorkloadProfile.
func (p *Profiler) ProfileWorkload(ctx context.Context, namespace, name, kind string) (*WorkloadProfile, error) {
	podSpec, err := p.getPodSpec(ctx, namespace, name, kind)
	if err != nil {
		return nil, err
	}

	profile := &WorkloadProfile{
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}

	// Pod-level security context
	if podSpec.SecurityContext != nil {
		sc := podSpec.SecurityContext
		profile.PodSecCtx = &PodSecuritySummary{
			RunAsNonRoot: sc.RunAsNonRoot,
			RunAsUser:    sc.RunAsUser,
			RunAsGroup:   sc.RunAsGroup,
			FSGroup:      sc.FSGroup,
		}
	}

	// Volumes
	for _, v := range podSpec.Volumes {
		profile.Volumes = append(profile.Volumes, summarizeVolume(v))
	}

	// Containers (init + regular)
	for _, c := range podSpec.InitContainers {
		profile.Containers = append(profile.Containers, profileContainer(c))
	}
	for _, c := range podSpec.Containers {
		profile.Containers = append(profile.Containers, profileContainer(c))
	}

	return profile, nil
}

func (p *Profiler) getPodSpec(ctx context.Context, namespace, name, kind string) (*corev1.PodSpec, error) {
	key := types.NamespacedName{Namespace: namespace, Name: name}
	switch kind {
	case "Deployment":
		obj := &appsv1.Deployment{}
		if err := p.client.Get(ctx, key, obj); err != nil {
			return nil, fmt.Errorf("fetching Deployment %s/%s: %w", namespace, name, err)
		}
		return &obj.Spec.Template.Spec, nil
	case "StatefulSet":
		obj := &appsv1.StatefulSet{}
		if err := p.client.Get(ctx, key, obj); err != nil {
			return nil, fmt.Errorf("fetching StatefulSet %s/%s: %w", namespace, name, err)
		}
		return &obj.Spec.Template.Spec, nil
	case "DaemonSet":
		obj := &appsv1.DaemonSet{}
		if err := p.client.Get(ctx, key, obj); err != nil {
			return nil, fmt.Errorf("fetching DaemonSet %s/%s: %w", namespace, name, err)
		}
		return &obj.Spec.Template.Spec, nil
	case "Pod":
		obj := &corev1.Pod{}
		if err := p.client.Get(ctx, key, obj); err != nil {
			return nil, fmt.Errorf("fetching Pod %s/%s: %w", namespace, name, err)
		}
		return &obj.Spec, nil
	default:
		return nil, fmt.Errorf("unsupported workload kind: %s", kind)
	}
}

func profileContainer(c corev1.Container) ContainerProfile {
	base, tag := ParseImageRef(c.Image)
	traits, known := LookupImage(c.Image)

	cp := ContainerProfile{
		Name:         c.Name,
		Image:        c.Image,
		ImageBase:    base,
		ImageTag:     tag,
		IsKnownImage: known,
	}

	if known {
		cp.KnownTraits = traits
		cp.WritablePaths = traits.WritablePaths
	}

	// Ports
	for _, port := range c.Ports {
		proto := string(port.Protocol)
		if proto == "" {
			proto = "TCP"
		}
		cp.Ports = append(cp.Ports, PortSummary{
			ContainerPort: port.ContainerPort,
			Protocol:      proto,
		})
		if port.ContainerPort < 1024 {
			cp.NeedsPrivPort = true
		}
	}

	// Volume mounts
	for _, vm := range c.VolumeMounts {
		cp.VolumeMounts = append(cp.VolumeMounts, vm.MountPath)
	}

	// Security context
	if c.SecurityContext != nil {
		cp.HasSecurityCtx = true
		sc := c.SecurityContext
		summary := &ContainerSecSummary{
			RunAsNonRoot:             sc.RunAsNonRoot,
			RunAsUser:                sc.RunAsUser,
			ReadOnlyRootFilesystem:   sc.ReadOnlyRootFilesystem,
			AllowPrivilegeEscalation: sc.AllowPrivilegeEscalation,
			Privileged:               sc.Privileged,
		}
		if sc.Capabilities != nil {
			for _, cap := range sc.Capabilities.Add {
				summary.Capabilities = append(summary.Capabilities, string(cap))
			}
			for _, cap := range sc.Capabilities.Drop {
				summary.DropCapabilities = append(summary.DropCapabilities, string(cap))
			}
		}
		cp.SecurityCtx = summary
	}

	// Resources
	reqs := c.Resources.Requests
	lims := c.Resources.Limits
	if len(reqs) > 0 || len(lims) > 0 {
		rs := &ResourceSummary{}
		if cpu, ok := reqs[corev1.ResourceCPU]; ok {
			rs.CPURequest = cpu.String()
		}
		if mem, ok := reqs[corev1.ResourceMemory]; ok {
			rs.MemoryRequest = mem.String()
		}
		if cpu, ok := lims[corev1.ResourceCPU]; ok {
			rs.CPULimit = cpu.String()
		}
		if mem, ok := lims[corev1.ResourceMemory]; ok {
			rs.MemoryLimit = mem.String()
		}
		cp.Resources = rs
	}

	return cp
}

func summarizeVolume(v corev1.Volume) VolumeSummary {
	vs := VolumeSummary{Name: v.Name}
	switch {
	case v.EmptyDir != nil:
		vs.Type = "emptyDir"
	case v.PersistentVolumeClaim != nil:
		vs.Type = "pvc"
	case v.ConfigMap != nil:
		vs.Type = "configMap"
	case v.Secret != nil:
		vs.Type = "secret"
	case v.HostPath != nil:
		vs.Type = "hostPath"
	case v.Projected != nil:
		vs.Type = "projected"
	case v.DownwardAPI != nil:
		vs.Type = "downwardAPI"
	case v.CSI != nil:
		vs.Type = "csi"
	default:
		vs.Type = "other"
	}
	return vs
}
