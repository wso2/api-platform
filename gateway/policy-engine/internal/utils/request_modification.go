package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// AddQueryParametersToPath adds query parameters to the given path
func AddQueryParametersToPath(path string, parameters map[string]string) string {
	// Parse the URL to handle existing query parameters
	parsedURL, err := url.Parse(path)
	if err != nil {
		// If URL parsing fails, fallback to simple append
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		// iterate over parameters and append
		for name, value := range parameters {
			path = fmt.Sprintf("%s%s%s=%s", path, separator, url.QueryEscape(name), url.QueryEscape(value))
			separator = "&"
		}
		return path
	}

	// Add the new query parameters to existing query parameters
	for name, value := range parameters {
		queryParams := parsedURL.Query()
		queryParams.Add(name, value)
		parsedURL.RawQuery = queryParams.Encode()
	}
	// Get the modified path with query parameters
	return parsedURL.String()
}

// RemoveQueryParametersFromPath removes specified query parameters from the given path
func RemoveQueryParametersFromPath(path string, parameters []string) string {
	// Parse the URL to handle existing query parameters
	parsedURL, err := url.Parse(path)
	if err != nil {
		// If URL parsing fails, return the original path
		return path
	}

	// Remove the specified query parameters
	queryParams := parsedURL.Query()
	for _, name := range parameters {
		queryParams.Del(name)
	}
	parsedURL.RawQuery = queryParams.Encode()
	// Get the modified path without the specified query parameters
	return parsedURL.String()
}
