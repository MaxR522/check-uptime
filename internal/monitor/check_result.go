package monitor

import (
	"fmt"
	"strings"
	"time"
)

type checkResult struct {
	id         int
	durationMs int64
	code       *int
	errMsg     *string
	errorBody  *string
	checkedAt  time.Time
	endpointID int
}

func deref[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}

func (r checkResult) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "[%s] ", r.checkedAt.Format(time.DateTime))
	fmt.Fprintf(&b, "id=%d ", r.id)

	if r.code != nil {
		fmt.Fprintf(&b, "status=%d ", *r.code)
	} else {
		b.WriteString("status=- ")
	}

	fmt.Fprintf(&b, "duration=%dms ", r.durationMs)
	fmt.Fprintf(&b, "error=%q ", deref(r.errMsg, ""))
	fmt.Fprintf(&b, "errorBody=%q", deref(r.errorBody, ""))
	fmt.Fprintf(&b, "id=%d ", r.id)
	fmt.Fprintf(&b, "endpoint=%d ", r.endpointID)

	return b.String()
}
