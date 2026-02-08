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

package testutils

// PtrString returns a pointer to the given string.
// Useful for constructing test objects with optional string fields.
func PtrString(s string) *string {
	return &s
}

// PtrInt returns a pointer to the given int.
func PtrInt(i int) *int {
	return &i
}

// PtrBool returns a pointer to the given bool.
func PtrBool(b bool) *bool {
	return &b
}

// PtrInt64 returns a pointer to the given int64.
func PtrInt64(i int64) *int64 {
	return &i
}

// PtrFloat64 returns a pointer to the given float64.
func PtrFloat64(f float64) *float64 {
	return &f
}
