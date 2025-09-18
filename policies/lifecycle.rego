package elava.lifecycle

import rego.v1

# Resources that haven't been seen recently might be deleted
decision := "flag" if {
	input.context.last_seen_days > 3
	input.resource.type in ["ec2", "rds", "elb"]
	not input.resource.tags.elava_blessed
}

action := "notify" if {
	decision == "flag"
	input.context.last_seen_days > 3
	input.context.last_seen_days <= 7
}

action := "investigate" if {
	decision == "flag"
	input.context.last_seen_days > 7
}

reason := sprintf("Resource not seen for %d days - might be terminated", [input.context.last_seen_days]) if {
	decision == "flag"
}

confidence := 0.9 if {
	input.context.last_seen_days > 7
}

confidence := 0.6 if {
	input.context.last_seen_days <= 7
}

risk := "high" if {
	decision == "flag"
	input.environment == "prod"
}

risk := "medium" if {
	decision == "flag"
	input.environment != "prod"
}

# Detect resources that change frequently (might be unstable)
decision := "flag" if {
	input.context.change_frequency > 10  # Changed more than 10 times
	input.context.resource_age_days < 7  # In less than a week
	input.resource.type == "ec2"
}

action := "notify" if {
	decision == "flag"
	input.context.change_frequency > 10
}

reason := sprintf("Resource changed %d times in %d days - potential instability", [input.context.change_frequency, input.context.resource_age_days]) if {
	decision == "flag"
	input.context.change_frequency > 10
}

confidence := 0.7 if {
	decision == "flag"
	input.context.change_frequency > 10
}

risk := "medium" if {
	decision == "flag"
	input.context.change_frequency > 10
}

# Detect development/test resources that are too old
decision := "flag" if {
	input.environment in ["dev", "test", "staging"]
	input.context.resource_age_days > 30
	not input.resource.tags.elava_blessed
}

action := "tag" if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
	input.context.resource_age_days > 30
	input.context.resource_age_days <= 60
}

action := "notify" if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
	input.context.resource_age_days > 60
}

reason := sprintf("Development resource running for %d days", [input.context.resource_age_days]) if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
}

confidence := 0.8 if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
	input.context.resource_age_days > 60
}

confidence := 0.5 if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
	input.context.resource_age_days <= 60
}

risk := "low" if {
	decision == "flag"
	input.environment in ["dev", "test", "staging"]
}