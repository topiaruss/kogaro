package validators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/distribution/reference"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/topiaruss/kogaro/internal/metrics"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ImageValidatorConfig holds configuration for image validation
type ImageValidatorConfig struct {
	// EnableImageValidation enables checking if images exist and are compatible
	EnableImageValidation bool
	// AllowMissingImages allows deployment even if images are not found
	AllowMissingImages bool
	// AllowArchitectureMismatch allows deployment even if image architecture doesn't match node
	AllowArchitectureMismatch bool
}

// ImageValidator validates container images
type ImageValidator struct {
	client               client.Client
	k8sClient            kubernetes.Interface
	log                  logr.Logger
	config               ImageValidatorConfig
	lastValidationErrors []ValidationError

	// For testing/mocking
	checkImageExistsFunc     func(reference.Reference) (bool, error)
	getImageArchitectureFunc func(reference.Reference) (string, error)
}

// NewImageValidator creates a new ImageValidator
func NewImageValidator(client client.Client, k8sClient kubernetes.Interface, log logr.Logger, config ImageValidatorConfig) *ImageValidator {
	return &ImageValidator{
		client:    client,
		k8sClient: k8sClient,
		log:       log,
		config:    config,
	}
}

// SetClient updates the client used by the validator
func (v *ImageValidator) SetClient(c client.Client) {
	v.client = c
}

// GetLastValidationErrors returns the errors from the last validation run
func (v *ImageValidator) GetLastValidationErrors() []ValidationError {
	return v.lastValidationErrors
}

// GetValidationType returns the validation type identifier for image validation
func (v *ImageValidator) GetValidationType() string {
	return "image_validation"
}

// ValidateCluster validates all container images in the cluster
func (v *ImageValidator) ValidateCluster(ctx context.Context) error {
	if !v.config.EnableImageValidation {
		return nil
	}

	// Get all nodes to check architecture compatibility
	nodes, err := v.k8sClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Get node architectures
	nodeArchitectures := make(map[string]bool)
	for _, node := range nodes.Items {
		arch := node.Status.NodeInfo.Architecture
		nodeArchitectures[arch] = true
	}

	// Validate all deployments
	var errors []ValidationError
	deploymentErrors, err := v.validateDeploymentImages(ctx, nodeArchitectures)
	if err != nil {
		return err
	}
	errors = append(errors, deploymentErrors...)

	// Validate all pods
	podErrors, err := v.validatePodImages(ctx, nodeArchitectures)
	if err != nil {
		return err
	}
	errors = append(errors, podErrors...)

	// Log validation results
	for _, validationErr := range errors {
		v.log.Info("validation error found",
			"validator_type", "image",
			"resource_type", validationErr.ResourceType,
			"resource_name", validationErr.ResourceName,
			"namespace", validationErr.Namespace,
			"validation_type", validationErr.ValidationType,
			"message", validationErr.Message,
		)

		metrics.ValidationErrors.WithLabelValues(
			validationErr.ResourceType,
			validationErr.ValidationType,
			validationErr.Namespace,
		).Inc()
	}

	v.log.Info("validation completed", "validator_type", "image", "total_errors", len(errors))
	
	// Store errors for CLI reporting
	v.lastValidationErrors = errors
	return nil
}

func (v *ImageValidator) validateDeploymentImages(ctx context.Context, nodeArchitectures map[string]bool) ([]ValidationError, error) {
	var errors []ValidationError
	var deployments appsv1.DeploymentList

	if err := v.client.List(ctx, &deployments); err != nil {
		return nil, fmt.Errorf("failed to list deployments: %w", err)
	}

	for _, deployment := range deployments.Items {
		// Validate main containers
		containerErrors := v.validateContainerImages(deployment.Spec.Template.Spec.Containers, "Deployment", deployment.Name, deployment.Namespace, nodeArchitectures)
		errors = append(errors, containerErrors...)

		// Validate init containers
		initContainerErrors := v.validateContainerImages(deployment.Spec.Template.Spec.InitContainers, "Deployment", deployment.Name, deployment.Namespace, nodeArchitectures)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ImageValidator) validatePodImages(ctx context.Context, nodeArchitectures map[string]bool) ([]ValidationError, error) {
	var errors []ValidationError
	var pods corev1.PodList

	if err := v.client.List(ctx, &pods); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		// Skip pods managed by controllers (they're validated via their controllers)
		if len(pod.OwnerReferences) > 0 {
			continue
		}

		// Validate main containers
		containerErrors := v.validateContainerImages(pod.Spec.Containers, "Pod", pod.Name, pod.Namespace, nodeArchitectures)
		errors = append(errors, containerErrors...)

		// Validate init containers
		initContainerErrors := v.validateContainerImages(pod.Spec.InitContainers, "Pod", pod.Name, pod.Namespace, nodeArchitectures)
		errors = append(errors, initContainerErrors...)
	}

	return errors, nil
}

func (v *ImageValidator) validateContainerImages(containers []corev1.Container, resourceType, resourceName, namespace string, nodeArchitectures map[string]bool) []ValidationError {
	var errors []ValidationError

	for _, container := range containers {
		// Parse image reference
		ref, err := reference.Parse(container.Image)
		if err != nil {
			errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "invalid_image_reference", "KOGARO-IMG-001", fmt.Sprintf("Container '%s' has invalid image reference: %s", container.Name, container.Image)).
				WithSeverity(SeverityError).
				WithRemediationHint("Fix the image reference format").
				WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
				WithDetail("container_name", container.Name).
				WithDetail("image", container.Image))
			continue
		}

		// Check if image exists
		imageExists, err := v.checkImageExists(ref)
		if err != nil {
			v.log.Error(err, "failed to check image existence", "image", container.Image)
			continue
		}

		if !imageExists {
			if !v.config.AllowMissingImages {
				errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "missing_image", "KOGARO-IMG-002", fmt.Sprintf("Container '%s' references non-existent image: %s", container.Name, container.Image)).
					WithSeverity(SeverityError).
					WithRemediationHint("Ensure the image exists in the registry or set allowMissingImages: true to proceed").
					WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
					WithDetail("container_name", container.Name).
					WithDetail("image", container.Image))
			} else {
				errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "missing_image_warning", "KOGARO-IMG-003", fmt.Sprintf("Container '%s' references non-existent image: %s (deployment allowed)", container.Name, container.Image)).
					WithSeverity(SeverityWarning).
					WithRemediationHint("Ensure the image will be available before deployment").
					WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
					WithDetail("container_name", container.Name).
					WithDetail("image", container.Image))
			}
		}

		// Check architecture compatibility
		if imageExists {
			arch, err := v.getImageArchitecture(ref)
			if err != nil {
				v.log.Error(err, "failed to get image architecture", "image", container.Image)
				continue
			}

			if !nodeArchitectures[arch] {
				if !v.config.AllowArchitectureMismatch {
					errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "architecture_mismatch", "KOGARO-IMG-004", fmt.Sprintf("Container '%s' image architecture (%s) is not compatible with any node in the cluster", container.Name, arch)).
						WithSeverity(SeverityError).
						WithRemediationHint("Use a multi-arch image or set allowArchitectureMismatch: true to proceed").
						WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
						WithDetail("container_name", container.Name).
						WithDetail("image", container.Image).
						WithDetail("image_architecture", arch).
						WithDetail("node_architectures", strings.Join(getKeys(nodeArchitectures), ", ")))
				} else {
					errors = append(errors, NewValidationErrorWithCode(resourceType, resourceName, namespace, "architecture_mismatch_warning", "KOGARO-IMG-005", fmt.Sprintf("Container '%s' image architecture (%s) is not compatible with any node in the cluster (deployment allowed)", container.Name, arch)).
						WithSeverity(SeverityWarning).
						WithRemediationHint("Ensure the image will be available for the node architecture before deployment").
						WithRelatedResources(fmt.Sprintf("Container/%s", container.Name)).
						WithDetail("container_name", container.Name).
						WithDetail("image", container.Image).
						WithDetail("image_architecture", arch).
						WithDetail("node_architectures", strings.Join(getKeys(nodeArchitectures), ", ")))
				}
			}
		}
	}

	return errors
}

func (v *ImageValidator) checkImageExists(ref reference.Reference) (bool, error) {
	if v.checkImageExistsFunc != nil {
		return v.checkImageExistsFunc(ref)
	}

	// Parse the reference using go-containerregistry
	tag, err := name.ParseReference(ref.String())
	if err != nil {
		return false, fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Create a context with timeout for registry operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Try to get the image descriptor to check if it exists
	_, err = remote.Get(tag, remote.WithContext(ctx))
	if err != nil {
		// Image doesn't exist or is not accessible
		return false, nil
	}

	return true, nil
}

func (v *ImageValidator) getImageArchitecture(ref reference.Reference) (string, error) {
	if v.getImageArchitectureFunc != nil {
		return v.getImageArchitectureFunc(ref)
	}

	// Parse the reference using go-containerregistry
	tag, err := name.ParseReference(ref.String())
	if err != nil {
		return "", fmt.Errorf("failed to parse image reference: %w", err)
	}

	// Create a context with timeout for registry operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get the image from the registry
	img, err := remote.Image(tag, remote.WithContext(ctx))
	if err != nil {
		return "", fmt.Errorf("failed to get image from registry: %w", err)
	}

	// Get the image config file which contains architecture information
	cfg, err := img.ConfigFile()
	if err != nil {
		return "", fmt.Errorf("failed to get image config: %w", err)
	}

	return cfg.Architecture, nil
}

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
