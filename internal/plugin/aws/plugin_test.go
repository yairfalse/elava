package aws

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yairfalse/elava/internal/filter"
)

func TestNewResource(t *testing.T) {
	p := &Plugin{
		region:    "us-east-1",
		accountID: "123456789012",
	}

	r := p.newResource("i-abc123", "ec2", "running", "my-instance")

	assert.Equal(t, "i-abc123", r.ID)
	assert.Equal(t, "ec2", r.Type)
	assert.Equal(t, "aws", r.Provider)
	assert.Equal(t, "us-east-1", r.Region)
	assert.Equal(t, "123456789012", r.Account)
	assert.Equal(t, "my-instance", r.Name)
	assert.Equal(t, "running", r.Status)
	assert.NotNil(t, r.Labels)
	assert.NotNil(t, r.Attrs)
	assert.WithinDuration(t, time.Now(), r.ScannedAt, time.Second)
}

func TestNewResource_EmptyName(t *testing.T) {
	p := &Plugin{
		region:    "eu-west-1",
		accountID: "987654321098",
	}

	r := p.newResource("vol-xyz", "ebs", "available", "")

	assert.Equal(t, "vol-xyz", r.ID)
	assert.Equal(t, "", r.Name)
}

func TestPluginName(t *testing.T) {
	p := &Plugin{}
	assert.Equal(t, "aws", p.Name())
}

func TestScanners(t *testing.T) {
	p := &Plugin{}
	scanners := p.scanners()

	expected := []string{
		"ec2", "rds", "elb", "s3", "eks", "asg", "lambda",
		"vpc", "subnet", "security_group", "dynamodb", "sqs",
		"ebs", "eip", "nat_gateway", "iam_role", "ecs",
		"route53", "cloudwatch_logs", "sns", "cloudfront",
		"elasticache", "secretsmanager", "acm", "apigateway",
		"kinesis", "redshift", "stepfunctions", "glue",
		"opensearch", "msk",
	}

	// Verify we have all expected scanners
	assert.Len(t, scanners, len(expected))

	// Verify scanner names
	names := make(map[string]bool)
	for _, s := range scanners {
		names[s.name] = true
	}

	for _, name := range expected {
		assert.True(t, names[name], "missing scanner: %s", name)
	}
}

func TestPlugin_MaxConcurrencyField(t *testing.T) {
	// Verify the plugin struct accepts maxConcurrency configuration
	p := &Plugin{
		region:         "us-east-1",
		accountID:      "123456789012",
		maxConcurrency: 10,
	}
	assert.Equal(t, int64(10), p.maxConcurrency)
}

func TestPlugin_FilterField(t *testing.T) {
	// Verify the plugin struct accepts filter configuration
	f := filter.New([]string{"iam_role"}, map[string]string{"env": "prod"}, nil)
	p := &Plugin{
		region:    "us-east-1",
		accountID: "123456789012",
		filter:    f,
	}
	assert.NotNil(t, p.filter)
	assert.False(t, p.filter.ShouldScanType("iam_role"))
	assert.True(t, p.filter.ShouldScanType("ec2"))
}
