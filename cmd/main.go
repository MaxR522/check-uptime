package main

import (
	"context"

	"github.com/MaxR522/check-uptime/internal/monitor"
)

func main() {
	context := context.Background()
	checker := monitor.NewChecker(nil)

	checker.RunChecker(context)
}
