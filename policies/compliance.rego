package elava.compliance

import rego.v1

# Resources must have required tags
decision := "require_approval" if {
	input.resource.tags.elava_owner == ""
	input.resource.tags.team == ""
	input.resource.tags.environment == ""
	input.resource.type in ["ec2", "rds", "elb", "s3"]
}

action := "tag" if {
	decision == "require_approval"
}

reason := "Resource missing required tags (owner, team, or environment)" if {
	decision == "require_approval"
}

confidence := 0.95 if {
	decision == "require_approval"
}

risk := "medium" if {
	decision == "require_approval"
}

# Production resources must have specific tags
decision := "flag" if {
	input.environment == "prod"
	input.resource.tags.contact == ""
	input.resource.type in ["ec2", "rds", "elb"]
}

action := "notify" if {
	decision == "flag"
	input.environment == "prod"
	input.resource.tags.contact == ""
}

reason := "Production resource missing contact information" if {
	decision == "flag"
	input.environment == "prod"
	input.resource.tags.contact == ""
}

confidence := 0.9 if {
	decision == "flag"
	input.environment == "prod"
}

risk := "high" if {
	decision == "flag"
	input.environment == "prod"
}

# Resources with conflicting tags
decision := "flag" if {
	input.resource.tags.environment == "prod"
	contains(input.resource.tags.name, "test")
}

decision := "flag" if {
	input.resource.tags.environment == "prod"
	contains(input.resource.tags.name, "dev")
}

action := "notify" if {
	decision == "flag"
	input.resource.tags.environment == "prod"
	(contains(input.resource.tags.name, "test") or contains(input.resource.tags.name, "dev"))
}

reason := "Resource has conflicting tags (prod environment but test/dev in name)" if {
	decision == "flag"
	input.resource.tags.environment == "prod"
	(contains(input.resource.tags.name, "test") or contains(input.resource.tags.name, "dev"))
}

confidence := 0.8 if {
	decision == "flag"
	input.resource.tags.environment == "prod"
}

risk := "medium" if {
	decision == "flag"
	input.resource.tags.environment == "prod"
}

# Long-running resources without generation tag
decision := "flag" if {
	input.context.resource_age_days > 90
	input.resource.tags.elava_generation == ""
	input.resource.type in ["ec2", "rds"]
}

action := "tag" if {
	decision == "flag"
	input.context.resource_age_days > 90
	input.resource.tags.elava_generation == ""
}

reason := sprintf("Long-running resource (%d days) without generation tag", [input.context.resource_age_days]) if {
	decision == "flag"
	input.context.resource_age_days > 90
}

confidence := 0.7 if {
	decision == "flag"
	input.context.resource_age_days > 90
}

risk := "low" if {
	decision == "flag"
	input.context.resource_age_days > 90
}