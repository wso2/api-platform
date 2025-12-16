package publishers

import "github.com/policy-engine/policy-engine/internal/analytics/dto"

// Publisher represents an analytics publisher.
type Publisher interface {
	Publish(event *dto.Event)
}
