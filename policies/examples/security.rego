package elava.policies.security

import future.keywords.if
import future.keywords.in

# Default: allow resources
default allow := true

# Flag security groups with overly permissive rules
flag if {
    input.resource.type == "security_group"
    has_public_ingress
}

# Notify about unencrypted storage
notify if {
    is_storage_resource
    not is_encrypted
}

# Check for public ingress (0.0.0.0/0)
has_public_ingress if {
    input.resource.type == "security_group"
    some rule in input.resource.config.ingress_rules
    rule.cidr == "0.0.0.0/0"
}

# Identify storage resources
is_storage_resource if {
    input.resource.type in ["ebs_volume", "s3_bucket", "rds"]
}

# Check encryption status
is_encrypted if {
    input.resource.config.encrypted == true
}

# Build decision
decision := {
    "action": action,
    "reason": reason,
    "confidence": confidence,
    "metadata": metadata
}

# Determine action
action := "flag" if {
    input.resource.type == "security_group"
    has_public_ingress
}

action := "notify" if {
    is_storage_resource
    not is_encrypted
    not action == "flag"
}

action := "ignore" if {
    not has_public_ingress
    not (is_storage_resource and not is_encrypted)
}

# Provide reason
reason := "Security group allows public access (0.0.0.0/0)" if {
    input.resource.type == "security_group"
    has_public_ingress
}

reason := sprintf("Unencrypted %s detected", [input.resource.type]) if {
    is_storage_resource
    not is_encrypted
    not has_public_ingress
}

reason := "Security checks passed" if {
    not has_public_ingress
    not (is_storage_resource and not is_encrypted)
}

# Confidence levels
confidence := 0.95 if has_public_ingress
confidence := 0.85 if {
    not has_public_ingress
    is_storage_resource
    not is_encrypted
}
confidence := 1.0 if {
    not has_public_ingress
    not (is_storage_resource and not is_encrypted)
}

# Metadata for enforcement
metadata := {
    "severity": "high",
    "tags_to_add": {"security:risk": "high"}
} if has_public_ingress

metadata := {
    "severity": "medium",
    "tags_to_add": {"security:encryption": "missing"}
} if {
    not has_public_ingress
    is_storage_resource
    not is_encrypted
}

metadata := {} if {
    not has_public_ingress
    not (is_storage_resource and not is_encrypted)
}
