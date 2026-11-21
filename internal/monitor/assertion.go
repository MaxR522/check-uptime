package monitor

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Assertion is one expected-value rule an endpoint response must satisfy.
// Check returns nil on pass, or an error whose message is the readable
// failure reason recorded on the check result and incident.
type Assertion interface {
	Check(resp *http.Response, body []byte, d time.Duration) error
}

// assertionParsers maps an expectation criteria to the function that parses
// and validates its expected value.
var assertionParsers = map[string]func(expected string) (Assertion, error){
	"status":            parseStatus,
	"max_response_time": parseMaxResponseTime,
	"body_contains":     parseBodyContains,
}

// parseAssertion builds the Assertion described by an expectation row.
// It is also the validation used before saving an expectation.
func parseAssertion(e expectation) (Assertion, error) {
	parse, ok := assertionParsers[e.criteria]
	if !ok {
		return nil, fmt.Errorf("unknown assertion %q", e.criteria)
	}
	a, err := parse(e.expectedValue)
	if err != nil {
		return nil, fmt.Errorf("assertion %s: %w", e.criteria, err)
	}
	return a, nil
}

var (
	statusListRe  = regexp.MustCompile(`^\d{3}(\s*,\s*\d{3})*$`)
	statusClassRe = regexp.MustCompile(`^(?i)[1-5]xx$`)
)

// statusAssertion matches the response status against an exact list
// (200,201), a class (2xx) or a regexp (^2\d\d$).
type statusAssertion struct {
	raw   string
	codes map[int]struct{}
	class int
	re    *regexp.Regexp
}

func parseStatus(expected string) (Assertion, error) {
	v := strings.TrimSpace(expected)
	a := &statusAssertion{raw: v}
	switch {
	case v == "":
		return nil, errors.New("empty expected status")
	case statusListRe.MatchString(v):
		a.codes = make(map[int]struct{})
		for s := range strings.SplitSeq(v, ",") {
			code, err := strconv.Atoi(strings.TrimSpace(s))
			if err != nil {
				return nil, fmt.Errorf("parse status code %q: %w", s, err)
			}
			if code < 100 || code > 599 {
				return nil, fmt.Errorf("status code %d out of range 100-599", code)
			}
			a.codes[code] = struct{}{}
		}
	case statusClassRe.MatchString(v):
		a.class = int(v[0] - '0')
	default:
		re, err := regexp.Compile(v)
		if err != nil {
			return nil, fmt.Errorf("compile status regexp: %w", err)
		}
		a.re = re
	}
	return a, nil
}

func (a *statusAssertion) Check(resp *http.Response, _ []byte, _ time.Duration) error {
	if resp == nil {
		return errors.New("no response")
	}
	code := resp.StatusCode
	switch {
	case a.codes != nil:
		if _, ok := a.codes[code]; !ok {
			return fmt.Errorf("status %d not in %s", code, a.raw)
		}
	case a.class != 0:
		if code/100 != a.class {
			return fmt.Errorf("status %d not in %s", code, a.raw)
		}
	default:
		if !a.re.MatchString(strconv.Itoa(code)) {
			return fmt.Errorf("status %d does not match %s", code, a.raw)
		}
	}
	return nil
}

// maxResponseTimeAssertion requires the response to arrive in less than max.
type maxResponseTimeAssertion struct {
	max time.Duration
}

func parseMaxResponseTime(expected string) (Assertion, error) {
	d, err := time.ParseDuration(strings.TrimSpace(expected))
	if err != nil {
		return nil, fmt.Errorf("parse max response time: %w", err)
	}
	if d <= 0 {
		return nil, fmt.Errorf("max response time %s must be positive", d)
	}
	return &maxResponseTimeAssertion{max: d}, nil
}

func (a *maxResponseTimeAssertion) Check(_ *http.Response, _ []byte, d time.Duration) error {
	if d >= a.max {
		return fmt.Errorf("response took %s, max %s", d.Round(time.Millisecond), a.max)
	}
	return nil
}

// bodyContainsAssertion requires the response body to contain want.
type bodyContainsAssertion struct {
	want []byte
}

func parseBodyContains(expected string) (Assertion, error) {
	if expected == "" {
		return nil, errors.New("empty expected body content")
	}
	return &bodyContainsAssertion{want: []byte(expected)}, nil
}

func (a *bodyContainsAssertion) Check(_ *http.Response, body []byte, _ time.Duration) error {
	if !bytes.Contains(body, a.want) {
		return fmt.Errorf("body does not contain %q", a.want)
	}
	return nil
}
