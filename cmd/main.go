package main

import (
	"context"

	"github.com/MaxR522/check-uptime/internal/monitor"
)

func main() {
	context := context.Background()
	service := monitor.NewService(nil)
	checker := monitor.NewChecker(service)

	checker.RunChecker(context)
}
