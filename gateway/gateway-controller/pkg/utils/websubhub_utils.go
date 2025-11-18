package utils

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
)

type WebsubhubUtilsService struct {
}

// NewAPIUtilsService creates a new API utilities service
func NewWebSubHubUtilsService() *WebsubhubUtilsService {
	return &WebsubhubUtilsService{}
}

const (
	WebSubHubURL = "http://localhost:9098/hub" // Replace with actual hub URL
)

// SendTopicRequestToHub sends a subscription request to the WebSub hub
func (s *APIUtilsService) SendTopicRequestToHub(topic string, mode string, gwHost string, logger *zap.Logger) error {
	// Prepare form data
	formData := fmt.Sprintf("hub.mode=%s&hub.topic=%s", mode, topic)

	// Create HTTP transport with proxy configuration
	proxyURL := "http://localhost:8082"
	parsedProxyURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("failed to parse proxy URL: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(parsedProxyURL),
	}

	// HTTP client with timeout and proxy
	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	// Retry on 404 Not Found (hub might not be ready immediately)
	const maxRetries = 5
	backoff := 500 * time.Millisecond
	var lastStatus int

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Create a fresh request each attempt (body readers are one-shot)
		req, err := http.NewRequest("POST", WebSubHubURL, strings.NewReader(formData))
		if err != nil {
			return fmt.Errorf("failed to create HTTP request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("failed to send HTTP request: %w", err)
		}

		// Ensure body is closed before next loop/return
		func() {
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				logger.Debug("Topic request sent to WebSubHub",
					zap.String("topic", topic),
					zap.String("mode", mode),
					zap.Int("status", resp.StatusCode))
				err = nil
				return
			}

			lastStatus = resp.StatusCode
		}()

		// Success path returned above
		if lastStatus == 0 {
			return nil
		}

		// Retry only on 404
		if lastStatus == http.StatusNotFound || lastStatus == http.StatusServiceUnavailable && attempt < maxRetries {
			time.Sleep(backoff)
			// Exponential backoff
			backoff *= 2
			lastStatus = 0
			continue
		}

		// Non-retryable status or retries exhausted
		return fmt.Errorf("WebSubHub returned non-success status: %d", lastStatus)
	}

	return fmt.Errorf("WebSubHub request failed after %d retries; last status: %d", maxRetries, lastStatus)
}
