package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yairfalse/elava/pkg/resource"
)

// mockPlugin implements Plugin for testing.
type mockPlugin struct {
	name      string
	resources []resource.Resource
	err       error
}

func (m *mockPlugin) Name() string {
	return m.name
}

func (m *mockPlugin) Scan(_ context.Context) ([]resource.Resource, error) {
	return m.resources, m.err
}

func TestRegister(t *testing.T) {
	Clear()
	defer Clear()

	p := &mockPlugin{name: "test"}
	Register(p)

	got, ok := Get("test")
	require.True(t, ok)
	assert.Equal(t, "test", got.Name())
}

func TestGet_NotFound(t *testing.T) {
	Clear()
	defer Clear()

	_, ok := Get("nonexistent")
	assert.False(t, ok)
}

func TestAll(t *testing.T) {
	Clear()
	defer Clear()

	Register(&mockPlugin{name: "aws"})
	Register(&mockPlugin{name: "gcp"})

	all := All()
	assert.Len(t, all, 2)
}

func TestAll_Empty(t *testing.T) {
	Clear()
	defer Clear()

	all := All()
	assert.Empty(t, all)
}

func TestNames(t *testing.T) {
	Clear()
	defer Clear()

	Register(&mockPlugin{name: "aws"})
	Register(&mockPlugin{name: "gcp"})

	names := Names()
	assert.Len(t, names, 2)
	assert.Contains(t, names, "aws")
	assert.Contains(t, names, "gcp")
}

func TestRegister_Overwrites(t *testing.T) {
	Clear()
	defer Clear()

	p1 := &mockPlugin{name: "aws", resources: []resource.Resource{{ID: "1"}}}
	p2 := &mockPlugin{name: "aws", resources: []resource.Resource{{ID: "2"}}}

	Register(p1)
	Register(p2)

	got, _ := Get("aws")
	resources, _ := got.Scan(context.Background())
	assert.Equal(t, "2", resources[0].ID)
}

func TestClear(t *testing.T) {
	Clear()

	Register(&mockPlugin{name: "aws"})
	assert.Len(t, All(), 1)

	Clear()
	assert.Empty(t, All())
}
