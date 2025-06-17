package models

// Component represents a component in the system.
type Component struct {
	ID       string
	Name     string
	ParentID *string // Pointer to string for nullable UUID
}
