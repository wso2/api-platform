/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package gateway

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// PrintJSONResponse prints a gateway HTTP response as pretty JSON when possible
// and falls back to the raw trimmed body otherwise. It closes the response body.
func PrintJSONResponse(resp *http.Response) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil
	}

	var jsonBody interface{}
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		prettyBody, marshalErr := json.MarshalIndent(jsonBody, "", "  ")
		if marshalErr == nil {
			fmt.Println(string(prettyBody))
			return nil
		}
	}

	fmt.Println(trimmed)
	return nil
}
