package adaptive

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func int32Ptr(i int32) *int32   { return &i }
func int64Ptr(i int64) *int64   { return &i }
func boolPtr(b bool) *bool      { return &b }

func TestProfileWorkload_Deployment(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "web"},
				},
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						RunAsUser:    int64Ptr(1000),
						FSGroup:      int64Ptr(1000),
					},
					Containers: []corev1.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
							Ports: []corev1.ContainerPort{
								{ContainerPort: 80, Protocol: corev1.ProtocolTCP},
							},
							SecurityContext: &corev1.SecurityContext{
								ReadOnlyRootFilesystem:   boolPtr(true),
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("128Mi"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "cache", MountPath: "/var/cache/nginx"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "cache",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	profiler := NewProfiler(fc)

	profile, err := profiler.ProfileWorkload(context.Background(), "default", "web", "Deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Kind != "Deployment" {
		t.Errorf("expected kind Deployment, got %s", profile.Kind)
	}
	if profile.Name != "web" {
		t.Errorf("expected name web, got %s", profile.Name)
	}

	// Pod security context
	if profile.PodSecCtx == nil {
		t.Fatal("expected pod security context")
	}
	if *profile.PodSecCtx.RunAsNonRoot != true {
		t.Error("expected RunAsNonRoot=true")
	}
	if *profile.PodSecCtx.RunAsUser != 1000 {
		t.Errorf("expected RunAsUser=1000, got %d", *profile.PodSecCtx.RunAsUser)
	}

	// Containers
	if len(profile.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(profile.Containers))
	}
	c := profile.Containers[0]
	if c.Name != "nginx" {
		t.Errorf("expected container name nginx, got %s", c.Name)
	}
	if c.ImageBase != "nginx" || c.ImageTag != "1.25" {
		t.Errorf("expected image (nginx, 1.25), got (%s, %s)", c.ImageBase, c.ImageTag)
	}
	if !c.IsKnownImage {
		t.Error("expected IsKnownImage=true for nginx")
	}
	if c.NeedsPrivPort != true {
		t.Error("expected NeedsPrivPort=true for port 80")
	}
	if !c.HasSecurityCtx {
		t.Error("expected HasSecurityCtx=true")
	}
	if c.SecurityCtx.ReadOnlyRootFilesystem == nil || *c.SecurityCtx.ReadOnlyRootFilesystem != true {
		t.Error("expected ReadOnlyRootFilesystem=true")
	}
	if len(c.SecurityCtx.DropCapabilities) != 1 || c.SecurityCtx.DropCapabilities[0] != "ALL" {
		t.Errorf("expected drop ALL, got %v", c.SecurityCtx.DropCapabilities)
	}

	// Resources
	if c.Resources == nil {
		t.Fatal("expected resources")
	}
	if c.Resources.CPURequest != "100m" {
		t.Errorf("expected CPU request 100m, got %s", c.Resources.CPURequest)
	}
	if c.Resources.MemoryLimit != "256Mi" {
		t.Errorf("expected memory limit 256Mi, got %s", c.Resources.MemoryLimit)
	}

	// Volume mounts
	if len(c.VolumeMounts) != 1 || c.VolumeMounts[0] != "/var/cache/nginx" {
		t.Errorf("expected volume mount /var/cache/nginx, got %v", c.VolumeMounts)
	}

	// Volumes
	if len(profile.Volumes) != 1 || profile.Volumes[0].Type != "emptyDir" {
		t.Errorf("expected emptyDir volume, got %v", profile.Volumes)
	}
}

func TestProfileWorkload_StatefulSet(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	ss := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: "data",
		},
		Spec: appsv1.StatefulSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "db"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "db"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "postgres",
							Image: "postgres:15",
						},
					},
				},
			},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ss).Build()
	profiler := NewProfiler(fc)

	profile, err := profiler.ProfileWorkload(context.Background(), "data", "db", "StatefulSet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile.Kind != "StatefulSet" {
		t.Errorf("expected kind StatefulSet, got %s", profile.Kind)
	}
	if len(profile.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(profile.Containers))
	}
	c := profile.Containers[0]
	if !c.IsKnownImage {
		t.Error("expected IsKnownImage=true for postgres")
	}
	if c.KnownTraits.DefaultUID != 999 {
		t.Errorf("expected UID 999, got %d", c.KnownTraits.DefaultUID)
	}
}

func TestProfileWorkload_UnknownImage(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "custom"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "custom"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "app",
							Image: "my-company/proprietary-app:v3.2.1",
						},
					},
				},
			},
		},
	}

	fc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	profiler := NewProfiler(fc)

	profile, err := profiler.ProfileWorkload(context.Background(), "default", "custom", "Deployment")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c := profile.Containers[0]
	if c.IsKnownImage {
		t.Error("expected IsKnownImage=false for unknown image")
	}
	if c.KnownTraits != nil {
		t.Error("expected nil KnownTraits for unknown image")
	}
	if c.ImageBase != "my-company/proprietary-app" {
		t.Errorf("expected base my-company/proprietary-app, got %s", c.ImageBase)
	}
	if c.ImageTag != "v3.2.1" {
		t.Errorf("expected tag v3.2.1, got %s", c.ImageTag)
	}
}

func TestProfileWorkload_NotFound(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	fc := fake.NewClientBuilder().WithScheme(scheme).Build()
	profiler := NewProfiler(fc)

	_, err := profiler.ProfileWorkload(context.Background(), "default", "missing", "Deployment")
	if err == nil {
		t.Fatal("expected error for missing deployment")
	}
}

func TestProfileWorkload_UnsupportedKind(t *testing.T) {
	scheme := runtime.NewScheme()
	fc := fake.NewClientBuilder().WithScheme(scheme).Build()
	profiler := NewProfiler(fc)

	_, err := profiler.ProfileWorkload(context.Background(), "default", "foo", "CronJob")
	if err == nil {
		t.Fatal("expected error for unsupported kind")
	}
}
