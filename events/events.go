package events

import "github.com/idena-network/idena-go/common/eventbus"

const (
	NodeCrashedEventID = eventbus.EventID("node-crashed")
)

type NodeCrashedEvent struct {
	Index int
}

func (e *NodeCrashedEvent) EventID() eventbus.EventID {
	return NodeCrashedEventID
}
