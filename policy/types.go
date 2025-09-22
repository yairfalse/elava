package policy

// Decision represents a policy evaluation result
type Decision struct {
	ResourceID string
	PolicyID   string
	Result     Result
	Reason     string
}

// Result types
type Result string

const (
	ResultAllow Result = "allow"
	ResultDeny  Result = "deny"
)
