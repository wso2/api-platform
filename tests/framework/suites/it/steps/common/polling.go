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
	"fmt"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// ResponseAttempt obtains one response for a polling operation.
type ResponseAttempt func(context.Context) (*httpx.Response, error)

// ResponseCondition reports whether a response satisfies a polling condition.
type ResponseCondition func(*httpx.Response) bool

// AwaitResponse polls an operation until condition holds.
//
// The attempt controls error classification. It should wrap transient transport
// errors with retry.Transient and return programming errors unchanged.
func AwaitResponse(
	ctx context.Context,
	attempt ResponseAttempt,
	condition ResponseCondition,
	what string,
) error {
	if attempt == nil {
		return fmt.Errorf("%s: response attempt is required", what)
	}
	if condition == nil {
		return fmt.Errorf("%s: response condition is required", what)
	}
	return retry.Await[*httpx.Response](ctx, retry.Options{},
		func(ctx context.Context) (*httpx.Response, error) { return attempt(ctx) },
		func(response *httpx.Response) bool { return condition(response) }, what)
}
