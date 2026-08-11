package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

type Store interface {
	GetWebsites(ctx context.Context) ([]Website, error)
	SaveIncident(ctx context.Context, incident Incident) error
}

type Checker struct {
	store          Store
	intervalMinute time.Duration
	client         *http.Client
}

type Result struct {
	Url        string
	StatusCode int
	Error      string
}

func NewChecker(store Store, intervalMinute time.Duration) *Checker {
	return &Checker{
		store:          store,
		intervalMinute: intervalMinute,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (checker *Checker) Start(ctx context.Context) {
	ticker := time.NewTicker(checker.intervalMinute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checker.checkAllAndReport(ctx)
		}
	}
}

func (checker *Checker) checkAllAndReport(ctx context.Context) {
	websites, err := checker.store.GetWebsites(ctx)
	if err != nil {
		fmt.Printf("There is an error when getting all websites. Error details: %s", err.Error())
		return
	}

	channels := make(chan Result, len(websites))

	for _, w := range websites {
		go func(url string) {
			res := checker.check(url)
			channels <- res
		}(w.URL)
	}

	for i := 0; i < len(websites); i++ {
		res := <-channels
		if res.Error != "" {
			incident := NewIncident(res.Error, res.StatusCode)
			err = checker.store.SaveIncident(ctx, incident)
			if err != nil {
				fmt.Printf("There is an error when saving incident. Error details: %s", err.Error())
				continue
			}
			fmt.Printf("Incident saved for the URL %s", res.Url)
		}

		fmt.Printf("No problem found for the url: %s", res.Url)
	}

	defer close(channels)
}

func (checker *Checker) check(url string) Result {
	resp, err := checker.client.Get(url)

	if err != nil {
		return Result{
			Url:        url,
			StatusCode: 0,
			Error:      err.Error(),
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return Result{
			Url:        url,
			StatusCode: resp.StatusCode,
			Error:      fmt.Sprintf("HTTP status code %d", resp.StatusCode),
		}
	}

	return Result{
		Url:        url,
		StatusCode: resp.StatusCode,
		Error:      "",
	}
}
