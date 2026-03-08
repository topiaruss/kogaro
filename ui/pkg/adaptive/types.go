package adaptive

// WorkloadProfile captures the live state of a Kubernetes workload
// for use by the adaptive fix engine.
type WorkloadProfile struct {
	Kind       string              `json:"kind"`
	Name       string              `json:"name"`
	Namespace  string              `json:"namespace"`
	Containers []ContainerProfile  `json:"containers"`
	PodSecCtx  *PodSecuritySummary `json:"podSecurityContext,omitempty"`
	Volumes    []VolumeSummary     `json:"volumes,omitempty"`
}

// PodSecuritySummary captures pod-level security context fields.
type PodSecuritySummary struct {
	RunAsNonRoot *bool  `json:"runAsNonRoot,omitempty"`
	RunAsUser    *int64 `json:"runAsUser,omitempty"`
	RunAsGroup   *int64 `json:"runAsGroup,omitempty"`
	FSGroup      *int64 `json:"fsGroup,omitempty"`
}

// VolumeSummary captures the name and type of a volume.
type VolumeSummary struct {
	Name string `json:"name"`
	Type string `json:"type"` // "emptyDir", "pvc", "configMap", "secret", etc.
}

// ContainerProfile captures per-container details relevant to fix generation.
type ContainerProfile struct {
	Name           string               `json:"name"`
	Image          string               `json:"image"`
	ImageBase      string               `json:"imageBase"`    // "nginx", "bitnami/postgresql"
	ImageTag       string               `json:"imageTag"`
	IsKnownImage   bool                 `json:"isKnownImage"`
	Ports          []PortSummary        `json:"ports,omitempty"`
	VolumeMounts   []string             `json:"volumeMounts,omitempty"` // mount paths
	HasSecurityCtx bool                 `json:"hasSecurityContext"`
	SecurityCtx    *ContainerSecSummary `json:"securityContext,omitempty"`
	Resources      *ResourceSummary     `json:"resources,omitempty"`
	WritablePaths  []string             `json:"writablePaths,omitempty"` // from knowledge base
	NeedsPrivPort  bool                 `json:"needsPrivPort"`
	KnownTraits    *ImageTraits         `json:"knownTraits,omitempty"`
}

// ContainerSecSummary captures container-level security context fields.
type ContainerSecSummary struct {
	RunAsNonRoot             *bool    `json:"runAsNonRoot,omitempty"`
	RunAsUser                *int64   `json:"runAsUser,omitempty"`
	ReadOnlyRootFilesystem   *bool    `json:"readOnlyRootFilesystem,omitempty"`
	AllowPrivilegeEscalation *bool    `json:"allowPrivilegeEscalation,omitempty"`
	Privileged               *bool    `json:"privileged,omitempty"`
	Capabilities             []string `json:"capabilities,omitempty"`     // added caps
	DropCapabilities         []string `json:"dropCapabilities,omitempty"`
}

// ResourceSummary captures CPU/memory requests and limits.
type ResourceSummary struct {
	CPURequest    string `json:"cpuRequest,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	CPULimit      string `json:"cpuLimit,omitempty"`
	MemoryLimit   string `json:"memoryLimit,omitempty"`
}

// PortSummary captures a container port and its protocol.
type PortSummary struct {
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// ImageTraits describes known characteristics of a container image.
type ImageTraits struct {
	DefaultUID        int64    `json:"defaultUID"`
	DefaultGID        int64    `json:"defaultGID"`
	WritablePaths     []string `json:"writablePaths"`
	NeedsCapabilities []string `json:"needsCapabilities,omitempty"`
	IsDistroless      bool     `json:"isDistroless,omitempty"`
	SafeForReadOnlyFS bool     `json:"safeForReadOnlyFS"`
	EnvPrefix         string   `json:"envPrefix,omitempty"`
	Notes             string   `json:"notes,omitempty"`
}

// FixOption represents one possible fix approach with risk assessment.
type FixOption struct {
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Risk        string   `json:"risk"` // "low", "medium", "high"
	Warnings    []string `json:"warnings,omitempty"`
	Commands    []FixCmd `json:"commands"`
	Rollback    []FixCmd `json:"rollback,omitempty"`
}

// FixCmd is a single command within a fix option.
type FixCmd struct {
	Label       string `json:"label"`
	Command     string `json:"command"`
	Destructive bool   `json:"destructive,omitempty"`
}
