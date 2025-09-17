package elava.ownership

import rego.v1

# Require ownership tags
decision := "require_approval" if {
	input.resource.tags.elava_owner == ""
	input.resource.type in ["ec2", "rds", "elb", "s3"]
	input.context.resource_age_days > 1
}

action := "tag" if {
	decision == "require_approval"
	input.resource.tags.elava_owner == ""
}

reason := sprintf("%s resource missing owner tag after %d days", [input.resource.type, input.context.resource_age_days]) if {
	decision == "require_approval"
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 1
}

confidence := 0.8 if {
	decision == "require_approval"
	input.resource.tags.elava_owner == ""
}

risk := "medium" if {
	decision == "require_approval"
	input.resource.tags.elava_owner == ""
}

# Orphan detection
decision := "flag" if {
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 30
	not input.resource.tags.temporary
	input.resource.type in ["ec2", "rds", "elb", "s3"]
}

action := "notify" if {
	decision == "flag"
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 30
}

reason := sprintf("Orphaned %s resource running for %d days without owner", [input.resource.type, input.context.resource_age_days]) if {
	decision == "flag"
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 30
}

confidence := 0.85 if {
	decision == "flag"
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 30
}

risk := "high" if {
	decision == "flag"
	input.resource.tags.elava_owner == ""
	input.context.resource_age_days > 30
}