package dto

// Label represents a label object in the DevPortal.
type Label struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName,omitempty"`
}

// LabelList is a convenience type for JSON arrays of labels.
type LabelList []Label
