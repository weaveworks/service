package users

// This file amends the generated struct by protobuf in users.pb.go

// Deleted returns true if this organization has been deleted.
func (t *Team) Deleted() bool {
	if t.DeletedAt == nil {
		return false
	}
	return !t.DeletedAt.IsZero()
}

// FormatCreatedAt formats a timestamp.
func (t *Team) FormatCreatedAt() string {
	return formatTimestamp(t.CreatedAt)
}

// FormatDeletedAt formats a timestamp.
func (t *Team) FormatDeletedAt() string {
	if t.DeletedAt == nil {
		return ""
	}
	return formatTimestamp(*t.DeletedAt)
}
