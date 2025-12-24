package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResourceKey(t *testing.T) {
	r := Resource{
		ID:       "i-abc123",
		Provider: "aws",
		Region:   "us-east-1",
		Account:  "123456789012",
	}

	key := ResourceKey(r)
	assert.Equal(t, "i-abc123|aws|us-east-1|123456789012", key)
}

func TestResourceKey_DifferentRegions(t *testing.T) {
	r1 := Resource{
		ID:       "vpc-default",
		Provider: "aws",
		Region:   "us-east-1",
		Account:  "123456789012",
	}
	r2 := Resource{
		ID:       "vpc-default",
		Provider: "aws",
		Region:   "us-west-2",
		Account:  "123456789012",
	}

	// Same ID but different regions should produce different keys
	assert.NotEqual(t, ResourceKey(r1), ResourceKey(r2))
}

func TestDiffType_Constants(t *testing.T) {
	assert.Equal(t, DiffType("added"), DiffAdded)
	assert.Equal(t, DiffType("deleted"), DiffDeleted)
	assert.Equal(t, DiffType("modified"), DiffModified)
}
