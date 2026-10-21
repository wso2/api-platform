/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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

package signing

import (
	"errors"
	"fmt"
)

// ReservedHeaders are set only by the signing service. alg and kid always
// describe the key that actually signed; jku is set only through WithJKU so it
// can only ever point at the operator-configured JWKS; crit and b64 would
// change how a verifier must process the token, which no policy needs.
var ReservedHeaders = map[string]bool{"alg": true, "kid": true, "jku": true, "crit": true, "b64": true}

// Options is the resolved protected-header customisation of a JWS, as the
// Service receives it.
type Options struct {
	// Type is the "typ" header; empty means none.
	Type string
	// Headers are extra protected header parameters. None is reserved.
	Headers map[string]any
	// JKU asks for the "jku" header, set to the configured public JWKS URL.
	JKU bool
}

type options struct {
	Options
	err error
}

// Option customises the protected header of a JWS.
type Option func(*options)

// WithType sets the "typ" header, e.g. "JWT" (the SignJWT default) or an
// application-specific media type.
func WithType(typ string) Option {
	return func(o *options) { o.Type = typ }
}

// WithHeader adds a protected header parameter. alg, kid, jku, crit and b64
// are reserved; setting one makes the signing call fail.
func WithHeader(name string, value any) Option {
	return func(o *options) {
		if name == "typ" {
			o.err = errors.New(`use WithType to set the "typ" header`)
			return
		}
		if ReservedHeaders[name] {
			o.err = fmt.Errorf("header %q is set by the gateway signing service and cannot be overridden", name)
			return
		}
		if o.Headers == nil {
			o.Headers = map[string]any{}
		}
		o.Headers[name] = value
	}
}

// WithJKU adds the "jku" header set to the operator-configured public JWKS
// URL ([signing] public_jwks_url). It is a no-op when none is configured.
func WithJKU() Option {
	return func(o *options) { o.JKU = true }
}

// ResolveOptions applies opts in order and returns the result, or the first
// error an option recorded.
func ResolveOptions(opts ...Option) (Options, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
		if o.err != nil {
			return Options{}, o.err
		}
	}
	return o.Options, nil
}
