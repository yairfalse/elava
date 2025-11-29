package emitter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yairfalse/elava/pkg/resource"
)

// mockEmitter implements Emitter for testing.
type mockEmitter struct {
	emitCalls  int
	closeCalls int
	emitErr    error
	closeErr   error
	results    []resource.ScanResult
}

func (m *mockEmitter) Emit(_ context.Context, result resource.ScanResult) error {
	m.emitCalls++
	m.results = append(m.results, result)
	return m.emitErr
}

func (m *mockEmitter) Close() error {
	m.closeCalls++
	return m.closeErr
}

func TestMultiEmitter_Emit(t *testing.T) {
	e1 := &mockEmitter{}
	e2 := &mockEmitter{}
	multi := NewMultiEmitter(e1, e2)

	result := resource.ScanResult{
		Provider:  "aws",
		Region:    "us-east-1",
		Resources: []resource.Resource{{ID: "i-123"}},
		Duration:  time.Second,
	}

	err := multi.Emit(context.Background(), result)

	require.NoError(t, err)
	assert.Equal(t, 1, e1.emitCalls)
	assert.Equal(t, 1, e2.emitCalls)
	assert.Len(t, e1.results, 1)
	assert.Len(t, e2.results, 1)
}

func TestMultiEmitter_Emit_Error(t *testing.T) {
	e1 := &mockEmitter{emitErr: errors.New("emit failed")}
	e2 := &mockEmitter{}
	multi := NewMultiEmitter(e1, e2)

	err := multi.Emit(context.Background(), resource.ScanResult{})

	assert.Error(t, err)
	assert.Equal(t, 1, e1.emitCalls)
	assert.Equal(t, 0, e2.emitCalls) // Should stop on first error
}

func TestMultiEmitter_Close(t *testing.T) {
	e1 := &mockEmitter{}
	e2 := &mockEmitter{}
	multi := NewMultiEmitter(e1, e2)

	err := multi.Close()

	require.NoError(t, err)
	assert.Equal(t, 1, e1.closeCalls)
	assert.Equal(t, 1, e2.closeCalls)
}

func TestMultiEmitter_Close_Error(t *testing.T) {
	e1 := &mockEmitter{closeErr: errors.New("close failed")}
	e2 := &mockEmitter{}
	multi := NewMultiEmitter(e1, e2)

	err := multi.Close()

	assert.Error(t, err)
	assert.Equal(t, 1, e1.closeCalls)
	assert.Equal(t, 0, e2.closeCalls) // Should stop on first error
}

func TestMultiEmitter_Empty(t *testing.T) {
	multi := NewMultiEmitter()

	err := multi.Emit(context.Background(), resource.ScanResult{})
	require.NoError(t, err)

	err = multi.Close()
	require.NoError(t, err)
}
