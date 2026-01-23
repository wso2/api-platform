/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package utils

import (
	"fmt"
	"net/url"
	"strings"
)

// AddQueryParametersToPath adds query parameters to the given path
func AddQueryParametersToPath(path string, parameters map[string][]string) string {
	// Parse the URL to handle existing query parameters
	parsedURL, err := url.Parse(path)
	if err != nil {
		// If URL parsing fails, fallback to simple append
		separator := "?"
		if strings.Contains(path, "?") {
			separator = "&"
		}
		// iterate over parameters and append all values
		for name, values := range parameters {
			for _, value := range values {
				path = fmt.Sprintf("%s%s%s=%s", path, separator, url.QueryEscape(name), url.QueryEscape(value))
				separator = "&"
			}
		}
		return path
	}

	// Get existing query parameters
	queryParams := parsedURL.Query()

	// Add the new query parameters to existing query parameters
	for name, values := range parameters {
		for _, value := range values {
			queryParams.Add(name, value)
		}
	}

	parsedURL.RawQuery = queryParams.Encode()
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
