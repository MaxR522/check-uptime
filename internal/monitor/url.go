package monitor

import (
	"net/http"
	"time"
)

type url struct {
	path                string
	intervalCheckMinute int
	assertions          []assertion
}

type assertion interface {
	Check(resp *http.Response, body []byte, duration time.Duration) error
}
