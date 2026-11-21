package monitor

import (
	"errors"
	"fmt"
	"net/http"
	"time"
)

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

// defaultStatusAssertion applies when an endpoint has no status expectation:
// any status below 400 passes.
type defaultStatusAssertion struct{}

func (defaultStatusAssertion) Check(resp *http.Response, _ []byte, _ time.Duration) error {
	if resp == nil {
		return errors.New("no response")
	}
	if resp.StatusCode >= 400 {
		return errors.New(resp.Status)
	}
	return nil
}

// buildAssertions parses every expectation. Without a status expectation it
// adds defaultStatusAssertion. All invalid expectations are reported.
func buildAssertions(exps []expectation) ([]Assertion, error) {
	var (
		out       = make([]Assertion, 0, len(exps)+1)
		errs      []error
		hasStatus bool
	)
	for _, e := range exps {
		a, err := parseAssertion(e)
		if err != nil {
			errs = append(errs, fmt.Errorf("expectation %d: %w", e.id, err))
			continue
		}
		if e.criteria == "status" {
			hasStatus = true
		}
		out = append(out, a)
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	if !hasStatus {
		out = append(out, defaultStatusAssertion{})
	}
	return out, nil
}
