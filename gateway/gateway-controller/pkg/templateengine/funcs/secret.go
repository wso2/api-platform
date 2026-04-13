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

package funcs

import "fmt"

func init() {
	Register(Func{
		Name: "secret",
		Fn: func(deps *Deps) any {
			return func(key string) (string, error) {
				if deps.SecretResolver == nil {
					return "", fmt.Errorf("secret resolver is not configured")
				}
				val, err := deps.SecretResolver.Resolve(key)
				if err != nil {
					return "", fmt.Errorf("failed to resolve secret %q: %w", key, err)
				}
				deps.Tracker.Track(val)
				return val, nil
			}
		},
	})
}
