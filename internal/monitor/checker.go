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
	service *Service
	client  *http.Client
}

func NewChecker(service *Service) *Checker {
	return &Checker{
		service: service,
		client: &http.Client{
			Timeout: 90 * time.Second,
		},
	}
}

func (c *Checker) RunChecker(ctx context.Context) {
	results, err := c.checkAllEndpoints(ctx)
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

			// to-do: save the resut
			fmt.Println(r)
		}
	}
}

func (c *Checker) checkAllEndpoints(ctx context.Context) (<-chan checkResult, error) {
	websites, err := c.service.GetWebsites(ctx)
	if err != nil {
		return nil, err
	}

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

				ctxWithTimeout, cancel := context.WithTimeout(ctx, time.Duration(endpoint.timeoutSec*float64(time.Second)))

				wg.Go(func() {
					defer cancel()
					result := checkEndpoint(ctxWithTimeout, c.client, fullURL, endpoint.id)
					select {
					case resultChan <- result:
					case <-ctx.Done():
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
		result.errorBody = new(string(body[:min(len(body), maxErrorBodyBytes)]))
	}

	result.durationMs = time.Since(startTimer).Milliseconds()

	return result
}
