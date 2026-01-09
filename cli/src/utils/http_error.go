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
	// If responder is Gateway Controller and 401, use the default message.
	if resp.StatusCode == http.StatusUnauthorized && responder == "Gateway Controller" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s failed (status %d) from %s: unauthorized.\n", operation, resp.StatusCode, responder))
		b.WriteString("Please check your credentials and try again.\n")
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

// CredentialSource indicates where authentication credentials were sourced from
type CredentialSource string

const (
	CredSourceEnv    CredentialSource = "env"    // Credentials from environment variables
	CredSourceConfig CredentialSource = "config" // Credentials from config file
)

// FormatHTTPErrorWithCredSource formats HTTP errors with credential-source-aware messaging for 401 errors.
func FormatHTTPErrorWithCredSource(operation string, resp *http.Response, responder string, authType string, credSource CredentialSource, gatewayName string) error {
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

	// Special handling for 401 Unauthorized from Gateway Controller
	if resp.StatusCode == http.StatusUnauthorized && responder == "Gateway Controller" {
		var b strings.Builder
		b.WriteString(fmt.Sprintf("%s failed (status %d) from %s: unauthorized.\n", operation, resp.StatusCode, responder))

		switch credSource {
		case CredSourceEnv:
			// Credentials came from environment variables
			b.WriteString("\nCredentials were sourced from environment variables.\n")
			b.WriteString("Please verify and re-export the correct values:\n")
			switch authType {
			case AuthTypeBasic:
				b.WriteString(fmt.Sprintf("  export %s=<username>\n", EnvGatewayUsername))
				b.WriteString(fmt.Sprintf("  export %s=<password>\n", EnvGatewayPassword))
			case AuthTypeBearer:
				b.WriteString(fmt.Sprintf("  export %s=<token>\n", EnvGatewayToken))
			}
		case CredSourceConfig:
			// Credentials came from config file
			b.WriteString("\nCredentials were sourced from the configuration file.\n")
			b.WriteString("Please either:\n")
			b.WriteString(fmt.Sprintf("  1. Re-add the gateway with correct credentials:\n"))
			b.WriteString(fmt.Sprintf("     ap gateway add --display-name %s --server <server_url> --auth %s\n", gatewayName, authType))
			b.WriteString(fmt.Sprintf("  2. Or export environment variables to override:\n"))
			switch authType {
			case AuthTypeBasic:
				b.WriteString(fmt.Sprintf("     export %s=<username>\n", EnvGatewayUsername))
				b.WriteString(fmt.Sprintf("     export %s=<password>\n", EnvGatewayPassword))
			case AuthTypeBearer:
				b.WriteString(fmt.Sprintf("     export %s=<token>\n", EnvGatewayToken))
			}
		}

		if bodyStr != "" {
			b.WriteString("\nServer response:\n")
			b.WriteString(bodyStr)
			b.WriteString("\n")
		}
		return fmt.Errorf(b.String())
	}

	// Default error formatting
	if responder == "" {
		return fmt.Errorf("%s failed (status %d): %s", operation, resp.StatusCode, bodyStr)
	}
	return fmt.Errorf("%s failed (status %d) from %s: %s", operation, resp.StatusCode, responder, bodyStr)
}
