/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package session

import "context"

// contextKey is unexported so no other package can construct a colliding key.
type contextKey struct{}

// WithContext returns a copy of ctx carrying the resolved session User.
func WithContext(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, contextKey{}, u)
}

// FromContext returns the session User stashed by the BFF's session
// middleware (server.sessionContext), if the request was authenticated. Any
// handler registered on the server's mux — a default route or a host-supplied
// one via server.Options.ExtraRoutes — can call this instead of re-deriving
// identity from the cookie itself.
func FromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}
