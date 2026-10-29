package monitor

import "time"

type incident struct {
	id         int
	reason     string
	resolvedAt time.Time
	startedAt  time.Time
	endpointID int
}
