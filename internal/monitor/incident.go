package monitor

import "time"

type Incident struct {
	Details    string
	Code       int
	ResolvedAt time.Time
}

func NewIncident(details string, code int) Incident {
	return Incident{
		Details: details,
		Code:    code,
	}
}
