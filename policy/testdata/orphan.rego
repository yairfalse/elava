package elava.orphan

# Simple policy: deny orphaned resources
deny contains msg if {
    not input.resource.tags.elava_owner
    msg := "Resource has no owner"
}