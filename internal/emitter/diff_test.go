package emitter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yairfalse/elava/pkg/resource"
)

func makeResource(id, status string, labels map[string]string) resource.Resource {
	return resource.Resource{
		ID:        id,
		Type:      "ec2",
		Provider:  "aws",
		Region:    "us-east-1",
		Account:   "123456789012",
		Name:      "test-" + id,
		Status:    status,
		Labels:    labels,
		Attrs:     make(map[string]string),
		ScannedAt: time.Now(),
	}
}

func TestDiffTracker_FirstScan(t *testing.T) {
	tracker := NewDiffTracker()
	resources := []resource.Resource{
		makeResource("i-001", "running", nil),
		makeResource("i-002", "running", nil),
	}

	// First scan should return nil (no diffs on baseline)
	diffs := tracker.ComputeDiff(resources)
	assert.Nil(t, diffs, "first scan should return nil")

	// Update state for next comparison
	tracker.Update(resources)
}

func TestDiffTracker_NoChanges(t *testing.T) {
	tracker := NewDiffTracker()
	resources := []resource.Resource{
		makeResource("i-001", "running", nil),
		makeResource("i-002", "running", nil),
	}

	// First scan - baseline
	tracker.ComputeDiff(resources)
	tracker.Update(resources)

	// Second scan - same resources
	diffs := tracker.ComputeDiff(resources)
	require.NotNil(t, diffs)
	assert.Empty(t, diffs, "identical resources should produce no diffs")
}

func TestDiffTracker_ResourceAdded(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline
	initial := []resource.Resource{
		makeResource("i-001", "running", nil),
	}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - new resource added
	updated := []resource.Resource{
		makeResource("i-001", "running", nil),
		makeResource("i-002", "running", nil), // new
	}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 1)
	assert.Equal(t, resource.DiffAdded, diffs[0].Type)
	assert.Equal(t, "i-002", diffs[0].Resource.ID)
	assert.Nil(t, diffs[0].Previous)
}

func TestDiffTracker_ResourceDeleted(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline with two resources
	initial := []resource.Resource{
		makeResource("i-001", "running", nil),
		makeResource("i-002", "running", nil),
	}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - one resource gone
	updated := []resource.Resource{
		makeResource("i-001", "running", nil),
	}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 1)
	assert.Equal(t, resource.DiffDeleted, diffs[0].Type)
	assert.Equal(t, "i-002", diffs[0].Resource.ID)
	assert.NotNil(t, diffs[0].Previous)
}

func TestDiffTracker_StatusChanged(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline
	initial := []resource.Resource{
		makeResource("i-001", "running", nil),
	}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - status changed
	updated := []resource.Resource{
		makeResource("i-001", "stopped", nil),
	}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 1)
	assert.Equal(t, resource.DiffModified, diffs[0].Type)
	assert.Equal(t, "i-001", diffs[0].Resource.ID)

	statusChange, ok := diffs[0].Changes["status"]
	require.True(t, ok, "should have status change")
	assert.Equal(t, "running", statusChange.Previous)
	assert.Equal(t, "stopped", statusChange.Current)
}

func TestDiffTracker_LabelsChanged(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline with labels
	initial := []resource.Resource{
		makeResource("i-001", "running", map[string]string{"env": "dev"}),
	}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - label changed
	updated := []resource.Resource{
		makeResource("i-001", "running", map[string]string{"env": "prod"}),
	}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 1)
	assert.Equal(t, resource.DiffModified, diffs[0].Type)

	_, hasLabelsChange := diffs[0].Changes["labels"]
	assert.True(t, hasLabelsChange, "should detect label change")
}

func TestDiffTracker_MultipleChanges(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline
	initial := []resource.Resource{
		makeResource("i-001", "running", nil),
		makeResource("i-002", "running", nil),
		makeResource("i-003", "running", nil),
	}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - mixed changes
	updated := []resource.Resource{
		makeResource("i-001", "stopped", nil), // modified
		// i-002 deleted
		makeResource("i-003", "running", nil), // unchanged
		makeResource("i-004", "running", nil), // added
	}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 3, "should have 3 diffs: modified, deleted, added")

	// Count by type
	counts := make(map[resource.DiffType]int)
	for _, d := range diffs {
		counts[d.Type]++
	}
	assert.Equal(t, 1, counts[resource.DiffModified])
	assert.Equal(t, 1, counts[resource.DiffDeleted])
	assert.Equal(t, 1, counts[resource.DiffAdded])
}

func TestDiffTracker_NameChanged(t *testing.T) {
	tracker := NewDiffTracker()

	// First scan - baseline
	r1 := makeResource("i-001", "running", nil)
	r1.Name = "old-name"
	initial := []resource.Resource{r1}
	tracker.ComputeDiff(initial)
	tracker.Update(initial)

	// Second scan - name changed
	r2 := makeResource("i-001", "running", nil)
	r2.Name = "new-name"
	updated := []resource.Resource{r2}
	diffs := tracker.ComputeDiff(updated)

	require.Len(t, diffs, 1)
	assert.Equal(t, resource.DiffModified, diffs[0].Type)

	nameChange, ok := diffs[0].Changes["name"]
	require.True(t, ok, "should have name change")
	assert.Equal(t, "old-name", nameChange.Previous)
	assert.Equal(t, "new-name", nameChange.Current)
}

func TestMapToJSON_Deterministic(t *testing.T) {
	// Verify JSON output is deterministic regardless of map iteration order
	m := map[string]string{"z": "last", "a": "first", "m": "middle"}

	// Call multiple times to verify consistent output
	result1 := mapToJSON(m)
	result2 := mapToJSON(m)

	assert.Equal(t, result1, result2, "mapToJSON should produce consistent output")
	assert.Equal(t, `{"a":"first","m":"middle","z":"last"}`, result1, "keys should be sorted")
}

func TestMapToJSON_NilMap(t *testing.T) {
	result := mapToJSON(nil)
	assert.Equal(t, "{}", result)
}
