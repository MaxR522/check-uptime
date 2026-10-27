package monitor

import (
	"net/http"
	"time"
)

type Checker struct {
	service Service
	client  *http.Client
}

type Result struct {
	url        string
	statusCode int
	error      string
}

func NewChecker(service Service) *Checker {
	return &Checker{
		service: service,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// - Each website has an array of url to be tested
// - Each URLs has a config intervalCheckMinute because sometimes, the interval check is not the same.
//      - example: a /health we need to check it every 5 minutes but a / need to be checked each 30 minutes
