package elava.cost

import rego.v1

# Delete stopped instances after 7 days
decision := "deny" if {
	input.resource.type == "ec2"
	input.resource.status == "stopped"
	days_stopped > 7
	not input.resource.tags.elava_blessed
}

action := "delete" if {
	decision == "deny"
	input.resource.type == "ec2"
	input.resource.status == "stopped"
	days_stopped > 7
}

reason := sprintf("EC2 instance stopped for %d days, exceeds 7 day limit", [days_stopped]) if {
	decision == "deny"
	input.resource.type == "ec2"
	input.resource.status == "stopped"
}

confidence := 0.9 if {
	decision == "deny"
	input.resource.type == "ec2"
	input.resource.status == "stopped"
}

risk := "medium" if {
	decision == "deny"
	input.resource.type == "ec2"
}

days_stopped := time.diff_days(time.now_ns(), input.resource.status_changed_at) if {
	input.resource.status_changed_at
}

days_stopped := 0 if {
	not input.resource.status_changed_at
}

# Flag expensive unused resources
decision := "flag" if {
	input.resource.type == "ec2"
	input.resource.instance_type in ["m5.xlarge", "m5.2xlarge", "c5.2xlarge"]
	input.context.resource_age_days > 14
	not input.resource.tags.elava_blessed
}

action := "tag" if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.instance_type in ["m5.xlarge", "m5.2xlarge", "c5.2xlarge"]
}

reason := sprintf("Large EC2 instance (%s) with low utilization for %d days", [input.resource.instance_type, input.context.resource_age_days]) if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.instance_type in ["m5.xlarge", "m5.2xlarge", "c5.2xlarge"]
}

confidence := 0.7 if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.instance_type in ["m5.xlarge", "m5.2xlarge", "c5.2xlarge"]
}

risk := "high" if {
	decision == "flag"
	input.resource.type == "ec2"
	input.resource.instance_type in ["m5.xlarge", "m5.2xlarge", "c5.2xlarge"]
}