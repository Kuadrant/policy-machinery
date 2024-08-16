package controller

import "k8s.io/apimachinery/pkg/runtime/schema"

type EventType int

const (
	CreateEvent EventType = iota
	UpdateEvent
	DeleteEvent
)

func (t *EventType) String() string {
	return [...]string{"create", "update", "delete"}[*t]
}

type ResourceEvent struct {
	Kind      schema.GroupKind
	EventType EventType
	OldObject Object
	NewObject Object
}

type ResourceEventMatcher struct {
	Kind            *schema.GroupKind
	EventType       *EventType
	ObjectNamespace string
	ObjectName      string
}
