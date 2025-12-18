package store

import "context"

// Secret is a secret with metadata.
type Metadata struct {
	Service  string   `dynamodbav:"service"`
	Inherits []string `dynamodbav:"inherits"`
}

// Store is an interface for a secret store.
type MetadataStore interface {
	Create(ctx context.Context, service string) (Metadata, error)
	Read(ctx context.Context, service string) (Metadata, error)
	SetInherits(ctx context.Context, service string, inherits []string) error
	AddInherits(ctx context.Context, service string, inherits []string) error
	DeleteInherits(ctx context.Context, service string, inherits []string) error
}
