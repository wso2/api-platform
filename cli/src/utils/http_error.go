package utils

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// FormatHTTPError reads the response body and returns a concise error message.
// It intentionally does not parse JSON and returns the raw response body trimmed.
func FormatHTTPError(operation string, resp *http.Response, responder string) error {
	if resp == nil {
		if responder == "" {
			return fmt.Errorf("%s failed: no response received", operation)
		}
		return fmt.Errorf("%s failed: no response received from %s", operation, responder)
	}

	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		if responder == "" {
			return fmt.Errorf("%s failed (status %d): failed to read response body: %v", operation, resp.StatusCode, err)
		}
		return fmt.Errorf("%s failed (status %d): failed to read response from %s: %v", operation, resp.StatusCode, responder, err)
	}
	bodyStr := strings.TrimSpace(string(bodyBytes))

	// Special-case 1
	// If responder is Gateway Controller and 401, suggest checking env vars.
	if resp.StatusCode == http.StatusUnauthorized && responder == "Gateway Controller" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s failed (status %d) from %s: unauthorized.\n", operation, resp.StatusCode, responder))
		b.WriteString("Please check that the following environment variables contain the correct values:\n")
		b.WriteString("  - WSO2AP_GW_USERNAME\n  - WSO2AP_GW_PASSWORD\n")
		if bodyStr != "" {
			b.WriteString("\nResponder response:\n")
			b.WriteString(bodyStr)
			b.WriteString("\n")
		}
		return fmt.Errorf(b.String())
	}

	if responder == "" {
		return fmt.Errorf("%s failed (status %d): %s", operation, resp.StatusCode, bodyStr)
	}
	return fmt.Errorf("%s failed (status %d) from %s: %s", operation, resp.StatusCode, responder, bodyStr)
}
