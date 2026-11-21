package monitor

type endpoint struct {
	id               int
	path             string
	websiteID        int
	intervalCheckMin int
	timeoutSec       float64
	enabled          bool
	expectations     []expectation
	incidents        []incident
}
