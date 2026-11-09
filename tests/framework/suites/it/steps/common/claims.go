/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cucumber/godog"
)

// ClaimsFromTable reads a two-column token-claim table. Values are expanded; a value that is a
// JSON array, object or quoted string is decoded, so a claim can be a list, a map, or a string
// Gherkin would otherwise trim; any other value is a string. A claim the table does not name is
// absent from the result, and a non-empty sub is required.
func ClaimsFromTable(ctx context.Context, table *godog.Table) (map[string]any, error) {
	if table == nil {
		return nil, errors.New("token claims table is required")
	}
	claims := make(map[string]any, len(table.Rows))
	for i, row := range table.Rows {
		if len(row.Cells) != 2 {
			return nil, fmt.Errorf("token claims row %d must have exactly two cells", i+1)
		}
		name := strings.TrimSpace(row.Cells[0].Value)
		if name == "" {
			return nil, fmt.Errorf("token claims row %d has an empty claim name", i+1)
		}
		if _, duplicate := claims[name]; duplicate {
			return nil, fmt.Errorf("token claim %q is given twice", name)
		}
		value, err := Expand(ctx, row.Cells[1].Value)
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimSpace(value)
		if strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, `"`) {
			var decoded any
			if err := json.Unmarshal([]byte(trimmed), &decoded); err != nil {
				return nil, fmt.Errorf("token claim %q is not valid JSON: %w", name, err)
			}
			claims[name] = decoded
			continue
		}
		claims[name] = value
	}
	if sub, _ := claims["sub"].(string); sub == "" {
		return nil, errors.New("token claims must include a sub")
	}
	return claims, nil
}
