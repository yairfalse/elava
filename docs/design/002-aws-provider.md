# Design Session: AWS Provider

## What problem are we solving?
Need an AWS provider that implements the CloudProvider interface to list, create, delete, and tag EC2 instances following Elava's stateless approach.

## What's the simplest solution?
- AWS provider struct with EC2 client
- List EC2 instances with filters
- Create instances from ResourceSpec
- Tag resources with elava:managed tags
- Delete by instance ID

## Can we break it into smaller functions?
Yes:
- `listInstances()` - Query AWS API (max 30 lines)
- `createInstance()` - Launch new instance (max 30 lines)
- `tagInstance()` - Apply tags (max 30 lines)
- `deleteInstance()` - Terminate instance (max 30 lines)
- `convertToResource()` - Map AWS instance to Resource (max 30 lines)

## What interfaces do we need?
```go
// AWS-specific interfaces
type EC2Client interface {
    DescribeInstances(ctx, input) (output, error)
    RunInstances(ctx, input) (output, error)
    TerminateInstances(ctx, input) (output, error)
    CreateTags(ctx, input) (output, error)
}

// Provider implements CloudProvider
type AWSProvider struct {
    client EC2Client
    region string
}
```

## What can go wrong?
- AWS credentials missing/invalid
- Rate limiting from AWS API
- Instance already terminated
- Tagging permissions missing
- Network connectivity issues

## Flow diagram
```
Config → AWSProvider → EC2Client → AWS API
                         ↓
Resource[] ← convertToResource() ← AWS Response
```

## Smallest testable units:
1. Mock EC2Client for testing
2. List instances with no filter
3. List instances with tag filter
4. Convert AWS instance to Resource
5. Create instance with tags
6. Delete instance by ID