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

func init() {
	Register(Func{
		Name: "default",
		Fn: func(_ *Deps) any {
			// Argument order is (fallback, val) so that piping works:
			//   {{ env "KEY" | default "fallback" }}
			return func(fallback, val string) string {
				if val != "" {
					return val
				}
				return fallback
			}
		},
	})
}
