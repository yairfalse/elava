package elava.waste

import rego.v1

# Detect stopped instances that have been idle too long
decision := "flag" if {
	input.resource.type == "ec2"
	input.resource.status == "stopped"
	input.context.last_seen_days > 7
	not input.resource.tags.elava_blessed
}

action := "notify" if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.status == "stopped"
	input.context.last_seen_days > 14
}

action := "delete" if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.status == "stopped"
	input.context.last_seen_days > 30
	not input.resource.tags.elava_blessed
}

reason := sprintf("EC2 instance stopped for %d days", [input.context.last_seen_days]) if {
	decision == "flag"
	input.resource.type == "ec2"
}

confidence := 0.9 if {
	decision == "flag"
	input.context.last_seen_days > 30
}

confidence := 0.7 if {
	decision == "flag"
	input.context.last_seen_days > 14
	input.context.last_seen_days <= 30
}

confidence := 0.5 if {
	decision == "flag"
	input.context.last_seen_days <= 14
}

risk := "low" if {
	decision == "flag"
	input.resource.status == "stopped"
}

# Detect unattached volumes (pure waste)
decision := "flag" if {
	input.resource.type == "ebs"
	input.resource.status == "available"  # Not attached to any instance
	input.context.resource_age_days > 3
}

action := "delete" if {
	decision == "flag"
	input.resource.type == "ebs"
	input.resource.status == "available"
	input.context.resource_age_days > 7
}

reason := sprintf("Unattached EBS volume for %d days", [input.context.resource_age_days]) if {
	decision == "flag"
	input.resource.type == "ebs"
}

confidence := 0.95 if {
	decision == "flag"
	input.resource.type == "ebs"
	input.resource.status == "available"
}

risk := "low" if {
	decision == "flag"
	input.resource.type == "ebs"
}

# Detect old snapshots without recent usage
decision := "flag" if {
	input.resource.type == "snapshot"
	input.context.resource_age_days > 90
	input.context.last_seen_days > 30  # Not accessed in 30 days
}

action := "delete" if {
	decision == "flag"
	input.resource.type == "snapshot"
	input.context.resource_age_days > 180
}

reason := sprintf("Snapshot %d days old, unused for %d days", [input.context.resource_age_days, input.context.last_seen_days]) if {
	decision == "flag"
	input.resource.type == "snapshot"
}

confidence := 0.8 if {
	decision == "flag"
	input.resource.type == "snapshot"
}

risk := "low" if {
	decision == "flag"
	input.resource.type == "snapshot"
}

# Detect oversized instances based on usage patterns
decision := "flag" if {
	input.resource.type == "ec2"
	input.resource.status == "running"
	contains(input.resource.metadata.instance_type, "xlarge")
	input.context.change_frequency < 2  # Rarely changes (stable workload)
	not input.resource.tags.elava_blessed
}

action := "tag" if {
	decision == "flag"
	input.resource.type == "ec2"
	contains(input.resource.metadata.instance_type, "xlarge")
}

reason := sprintf("Large instance type %s with stable workload - consider rightsizing", [input.resource.metadata.instance_type]) if {
	decision == "flag"
	input.resource.type == "ec2"
	contains(input.resource.metadata.instance_type, "xlarge")
}

confidence := 0.6 if {
	decision == "flag"
	contains(input.resource.metadata.instance_type, "xlarge")
}

risk := "medium" if {
	decision == "flag"
	input.resource.status == "running"
}