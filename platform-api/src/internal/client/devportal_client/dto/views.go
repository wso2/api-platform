package dto

// ViewRequest represents create/update of a view
type ViewRequest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels,omitempty"`
}

// ViewResponse represents a view returned by the DevPortal
type ViewResponse struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels"`
}

type ViewUpdateRequest struct {
	DisplayName string   `json:"displayName"`
	Labels      []string `json:"labels,omitempty"`
}
