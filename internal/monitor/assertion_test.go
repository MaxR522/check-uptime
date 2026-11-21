package monitor

import (
	"net/http"
	"testing"
	"time"
)

func TestParseAssertion(t *testing.T) {
	tests := []struct {
		name     string
		criteria string
		expected string
		wantErr  bool
	}{
		{"status single", "status", "200", false},
		{"status list", "status", "200,201", false},
		{"status list with spaces", "status", " 200, 204 ", false},
		{"status class", "status", "2xx", false},
		{"status class upper", "status", "3XX", false},
		{"status regexp", "status", `^2\d\d$`, false},
		{"max response time", "max_response_time", "250ms", false},
		{"body contains", "body_contains", "ok", false},
		{"unknown criteria", "header_equals", "x", true},
		{"empty status", "status", "  ", true},
		{"status out of range", "status", "999", true},
		{"status list out of range", "status", "200,099", true},
		{"invalid status regexp", "status", "(", true},
		{"invalid duration", "max_response_time", "fast", true},
		{"zero duration", "max_response_time", "0s", true},
		{"negative duration", "max_response_time", "-1s", true},
		{"empty body", "body_contains", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := parseAssertion(expectation{criteria: tt.criteria, expectedValue: tt.expected})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseAssertion(%q, %q) = %T, want error", tt.criteria, tt.expected, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseAssertion(%q, %q): %v", tt.criteria, tt.expected, err)
			}
		})
	}
}

// checkAssertion parses an assertion and runs it, failing the test on a parse error.
func checkAssertion(t *testing.T, criteria, expected string, resp *http.Response, body []byte, d time.Duration) error {
	t.Helper()
	a, err := parseAssertion(expectation{criteria: criteria, expectedValue: expected})
	if err != nil {
		t.Fatalf("parseAssertion(%q, %q): %v", criteria, expected, err)
	}
	return a.Check(resp, body, d)
}

func assertReason(t *testing.T, err error, want string) {
	t.Helper()
	switch {
	case want == "" && err != nil:
		t.Fatalf("Check() = %q, want pass", err)
	case want != "" && err == nil:
		t.Fatalf("Check() passed, want %q", want)
	case want != "" && err.Error() != want:
		t.Fatalf("Check() = %q, want %q", err, want)
	}
}

func TestStatusAssertion(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		resp     *http.Response
		want     string
	}{
		{"list pass", "200,201", &http.Response{StatusCode: 201}, ""},
		{"list fail", "200,201", &http.Response{StatusCode: 503}, "status 503 not in 200,201"},
		{"list with spaces pass", "200, 204", &http.Response{StatusCode: 204}, ""},
		{"class pass", "2xx", &http.Response{StatusCode: 299}, ""},
		{"class fail", "2xx", &http.Response{StatusCode: 301}, "status 301 not in 2xx"},
		{"regexp pass", `^2\d\d$`, &http.Response{StatusCode: 200}, ""},
		{"regexp fail", `^2\d\d$`, &http.Response{StatusCode: 404}, `status 404 does not match ^2\d\d$`},
		{"no response", "200", nil, "no response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAssertion(t, "status", tt.expected, tt.resp, nil, 0)
			assertReason(t, err, tt.want)
		})
	}
}

func TestMaxResponseTimeAssertion(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"under", 100 * time.Millisecond, ""},
		{"equal", 500 * time.Millisecond, "response took 500ms, max 500ms"},
		{"over", 812300 * time.Microsecond, "response took 812ms, max 500ms"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAssertion(t, "max_response_time", "500ms", &http.Response{StatusCode: 200}, nil, tt.d)
			assertReason(t, err, tt.want)
		})
	}
}

func TestBodyContainsAssertion(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{"contains", []byte(`{"status":"ok"}`), ""},
		{"missing", []byte(`{"status":"down"}`), `body does not contain "ok"`},
		{"empty body", nil, `body does not contain "ok"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkAssertion(t, "body_contains", "ok", &http.Response{StatusCode: 200}, tt.body, 0)
			assertReason(t, err, tt.want)
		})
	}
}
