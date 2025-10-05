package elava.policies.orphans

import future.keywords.if
import future.keywords.in

# Default: allow resources with proper tags
default allow := true

# Flag resources missing critical tags
flag if {
    missing_critical_tags
}

# Notify about resources without owner
notify if {
    missing_owner_tag
}

# Resource is missing critical tags
missing_critical_tags if {
    not input.resource.tags["elava:managed"]
}

# Resource is missing owner tag
missing_owner_tag if {
    not input.resource.tags["Owner"]
    not input.resource.tags["owner"]
}

# Build decision based on resource state
decision := {
    "action": action,
    "reason": reason,
    "confidence": confidence
}

# Determine action based on conditions
action := "flag" if missing_critical_tags
action := "notify" if {
    not missing_critical_tags
    missing_owner_tag
}
action := "ignore" if {
    not missing_critical_tags
    not missing_owner_tag
}

# Provide reason for decision
reason := "Missing elava:managed tag - resource is untracked" if missing_critical_tags
reason := "Missing Owner tag - unclear ownership" if {
    not missing_critical_tags
    missing_owner_tag
}
reason := "Resource properly tagged" if {
    not missing_critical_tags
    not missing_owner_tag
}

# Confidence based on tag completeness
confidence := 0.9 if missing_critical_tags
confidence := 0.7 if {
    not missing_critical_tags
    missing_owner_tag
}
confidence := 1.0 if {
    not missing_critical_tags
    not missing_owner_tag
}
