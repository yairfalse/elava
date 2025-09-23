package elava.orphan

# Simple policy: deny orphaned resources
deny contains msg if {
    input.resource.tags.owner == ""
    msg := "Resource has no owner"
}