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

package webhook

import "context"

// ctxDecryptorKey is the private context key used to carry the per-request
// field-decryptor from ReceivePerPortalEvent down to the event handlers. The
// value type is kept private so no caller outside this package can collide
// with the key.
type ctxDecryptorKey struct{}

// contextWithDecryptor attaches a decryptor to ctx so handlers reached via
// ReceivePerPortalEvent decrypt payload fields with the same per-portal secret
// that just verified the signature.
func contextWithDecryptor(ctx context.Context, d *Decryptor) context.Context {
	if d == nil {
		return ctx
	}
	return context.WithValue(ctx, ctxDecryptorKey{}, d)
}

// decryptorFor returns the decryptor to use for the current request. Per-portal
// deliveries carry the decryptor on the context; the legacy /webhook/events
// route has no context value and falls back to r.decryptor (the global
// field-decryptor built from the shared secret). r.decryptor is nil when the
// legacy route is disabled, in which case any payload field flagged encrypted
// on the per-portal path must have come with a non-nil ctx decryptor.
func (r *Receiver) decryptorFor(ctx context.Context) *Decryptor {
	if d, ok := ctx.Value(ctxDecryptorKey{}).(*Decryptor); ok && d != nil {
		return d
	}
	return r.decryptor
}
