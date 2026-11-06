package monitor

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const maxBodyBytes = 1 << 20
const maxErrorBodyBytes = 250

type Checker struct {
	service *service
	client  *http.Client
}

func NewChecker(store Store) *Checker {
	service := newService(store)

	return &Checker{
		service: service,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *Checker) RunChecker(ctx context.Context) {
	websites, err := c.service.GetWebsites(ctx)
	if err != nil {
		log.Println(err.Error())
		return
	}

	results, err := c.checkAllEndpoints(ctx, websites)
	if err != nil {
		log.Println(err.Error())
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case r, ok := <-results:
			if !ok {
				return
			}

			if r.errMsg != nil {
				// report incident
				newIncident := incident{
					id:         0,
					reason:     *r.errMsg,
					startedAt:  time.Now(),
					endpointID: r.endpointID,
				}

				fmt.Println(newIncident)
				c.recordIncident(ctx, newIncident)
			}

			c.service.SaveResult(ctx, r)
		}
	}
}

func (c *Checker) recordIncident(ctx context.Context, newIncident incident) error {
	// 1. check if the incident already exists for the endpointID
	// 2. the incident is different that the already reported
	// 3. save the incident
	existingIncident, err := c.service.GetIncident(ctx, newIncident.endpointID)
	if err != nil {
		return err
	}

	if existingIncident.reason == newIncident.reason {
		return fmt.Errorf("Already incident already created")
	}

	// c.service.SaveIncident(ctx, newIncident)
	return nil
}

func (c *Checker) checkAllEndpoints(ctx context.Context, websites []website) (<-chan checkResult, error) {
	resultChan := make(chan checkResult)

	go func() {
		defer close(resultChan)
		var wg sync.WaitGroup

		for _, web := range websites {
			for _, endpoint := range web.endpoints {
				if !endpoint.enabled {
					continue
				}

				fullURL, err := buildURL(web.baseURL, endpoint.path)
				if err != nil {
					log.Println(err)
					continue
				}

				if endpoint.intervalCheckMin <= 0 {
					log.Printf("invalid intervalCheckMin %d for %s", endpoint.intervalCheckMin, fullURL)
					continue
				}

				if endpoint.timeoutSec <= 0 {
					log.Printf("invalid timeoutSec %f for %s", endpoint.timeoutSec, fullURL)
					continue
				}

				wg.Go(func() {
					ticker := time.NewTicker(time.Duration(endpoint.intervalCheckMin) * time.Minute)
					defer ticker.Stop()

					runCheckEndpoint(ctx, c.client, endpoint, fullURL, resultChan)

					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							runCheckEndpoint(ctx, c.client, endpoint, fullURL, resultChan)
						}
					}
				})
			}
		}

		wg.Wait()
	}()

	return resultChan, nil
}

func buildURL(baseURL, path string) (string, error) {
	fullURL, err := url.JoinPath(baseURL, path)
	if err != nil {
		return "", fmt.Errorf("error building full path with %w", err)
	}

	return fullURL, nil
}

func runCheckEndpoint(ctx context.Context, client *http.Client, ep endpoint, targetURL string, resultChan chan<- checkResult) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(ep.timeoutSec*float64(time.Second)))
	result := checkEndpoint(ctxWithTimeout, client, targetURL, ep.id)
	cancel()
	select {
	case resultChan <- result:
	case <-ctx.Done():
	}
}

func checkEndpoint(ctx context.Context, client *http.Client, target string, endpointID int) checkResult {
	startTimer := time.Now()
	result := checkResult{checkedAt: startTimer, endpointID: endpointID}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		result.errMsg = new(err.Error())
		result.durationMs = time.Since(startTimer).Milliseconds()
		return result
	}

	resp, err := client.Do(req)
	if err != nil {
		result.errMsg = new(err.Error())
		result.durationMs = time.Since(startTimer).Milliseconds()

		return result
	}

	defer resp.Body.Close()

	result.code = &resp.StatusCode

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if readErr != nil {
		result.errMsg = new(readErr.Error())
		result.durationMs = time.Since(startTimer).Milliseconds()
		return result
	}

	if resp.StatusCode >= 400 {
		result.errBody = new(string(body[:min(len(body), maxErrorBodyBytes)]))
		result.errMsg = new(resp.Status)
	}

	result.durationMs = time.Since(startTimer).Milliseconds()

	return result
}
