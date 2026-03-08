package datasource

import (
	"context"
	"fmt"

	"github.com/topiaruss/kogaro/internal/validators"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// KogaroDataSource runs Kogaro validators directly against a kubeconfig.
type KogaroDataSource struct {
	client   client.Client
	registry *validators.ValidatorRegistry
}

// NewKogaroDataSource creates a data source that uses Kogaro's validators.
func NewKogaroDataSource(c client.Client, registry *validators.ValidatorRegistry) *KogaroDataSource {
	return &KogaroDataSource{
		client:   c,
		registry: registry,
	}
}

func (k *KogaroDataSource) Name() string {
	return "kogaro"
}

func (k *KogaroDataSource) Scan(ctx context.Context) ([]validators.ValidationError, error) {
	if err := k.registry.ValidateCluster(ctx); err != nil {
		return nil, fmt.Errorf("kogaro scan failed: %w", err)
	}

	var allErrors []validators.ValidationError
	for _, v := range k.registry.GetValidators() {
		allErrors = append(allErrors, v.GetLastValidationErrors()...)
	}
	return allErrors, nil
}

func (k *KogaroDataSource) IsAvailable(ctx context.Context) bool {
	return k.client != nil && k.registry != nil
}
