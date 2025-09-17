package elava.security

import rego.v1

# High-risk public resources
decision := "flag" if {
	input.resource.type == "s3"
	input.resource.public_read == true
	not input.resource.tags.approved_public
	input.environment == "prod"
}

action := "notify" if {
	decision == "flag"
	input.resource.type == "s3"
	input.resource.public_read == true
}

reason := "Public S3 bucket in production without approval tag" if {
	decision == "flag"
	input.resource.type == "s3"
	input.resource.public_read == true
	input.environment == "prod"
}

confidence := 0.95 if {
	decision == "flag"
	input.resource.type == "s3"
	input.resource.public_read == true
	input.environment == "prod"
}

risk := "high" if {
	decision == "flag"
	input.resource.type == "s3"
	input.resource.public_read == true
	input.environment == "prod"
}

# Security group violations
decision := "flag" if {
	input.resource.type == "ec2"
	some rule in input.resource.security_groups
	rule.from_port == 22
	rule.source == "0.0.0.0/0"
	input.environment == "prod"
}

action := "notify" if {
	decision == "flag"
	input.resource.type == "ec2"
	some rule in input.resource.security_groups
	rule.from_port == 22
	rule.source == "0.0.0.0/0"
}

reason := "EC2 instance allows SSH from anywhere (0.0.0.0/0) in production" if {
	decision == "flag"
	input.resource.type == "ec2"
	some rule in input.resource.security_groups
	rule.from_port == 22
	rule.source == "0.0.0.0/0"
	input.environment == "prod"
}

confidence := 0.9 if {
	decision == "flag"
	input.resource.type == "ec2"
	some rule in input.resource.security_groups
	rule.from_port == 22
	rule.source == "0.0.0.0/0"
}

risk := "high" if {
	decision == "flag"
	input.resource.type == "ec2"
	some rule in input.resource.security_groups
	rule.from_port == 22
	rule.source == "0.0.0.0/0"
	input.environment == "prod"
}