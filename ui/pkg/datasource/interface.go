package datasource

import (
	"context"

	"github.com/topiaruss/kogaro/internal/validators"
)

// DataSource provides validation errors from a source.
type DataSource interface {
	Name() string
	Scan(ctx context.Context) ([]validators.ValidationError, error)
	IsAvailable(ctx context.Context) bool
}
