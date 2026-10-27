package monitor

import "time"

type incident struct {
	details    string
	code       int
	resolvedAt time.Time
}

func NewIncident(details string, code int) incident {
	return incident{
		details: details,
		code:    code,
	}
}
