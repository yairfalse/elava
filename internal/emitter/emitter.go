// Package emitter defines the output interface for Elava.
package emitter

import (
	"context"

	"github.com/yairfalse/elava/pkg/resource"
)

// Emitter outputs scanned resources to a backend.
type Emitter interface {
	// Emit sends resources to the backend.
	Emit(ctx context.Context, result resource.ScanResult) error

	// Close cleans up resources.
	Close() error
}

// MultiEmitter fans out to multiple emitters.
type MultiEmitter struct {
	emitters []Emitter
}

// NewMultiEmitter creates an emitter that sends to multiple backends.
func NewMultiEmitter(emitters ...Emitter) *MultiEmitter {
	return &MultiEmitter{emitters: emitters}
}

// Emit sends to all emitters, returns first error.
func (m *MultiEmitter) Emit(ctx context.Context, result resource.ScanResult) error {
	for _, e := range m.emitters {
		if err := e.Emit(ctx, result); err != nil {
			return err
		}
	}
	return nil
}

// Close closes all emitters.
func (m *MultiEmitter) Close() error {
	for _, e := range m.emitters {
		if err := e.Close(); err != nil {
			return err
		}
	}
	return nil
}
