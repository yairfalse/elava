package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yairfalse/elava/internal/plugin"
	"github.com/yairfalse/elava/pkg/resource"
)

func TestHandleHealthz(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	handleHealthz(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestHandleReadyz_NoPlugins(t *testing.T) {
	// Clear any registered plugins
	plugin.Clear()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handleReadyz(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Equal(t, "no plugins registered", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))
}

func TestHandleReadyz_WithPlugins(t *testing.T) {
	// Register a mock plugin
	plugin.Clear()
	plugin.Register(&mockPlugin{})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	handleReadyz(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "ok", w.Body.String())
	assert.Equal(t, "text/plain; charset=utf-8", w.Header().Get("Content-Type"))

	// Cleanup
	plugin.Clear()
}

type mockPlugin struct{}

func (m *mockPlugin) Name() string { return "mock" }
func (m *mockPlugin) Scan(_ context.Context) ([]resource.Resource, error) {
	return nil, nil
}
