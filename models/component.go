package models

type Component struct {
	ID       int         `json:"id"`
	Name     string      `json:"name"`
	ParentID *int        `json:"parent_id,omitempty"`
	Children []Component `json:"children,omitempty"`
}
