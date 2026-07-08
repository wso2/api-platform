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

import "errors"

var (
	// ErrSignatureMissing indicates the signature header was absent. -> 401
	ErrSignatureMissing = errors.New("webhook signature header missing")
	// ErrSignatureInvalid indicates the HMAC did not match. -> 401
	ErrSignatureInvalid = errors.New("webhook signature invalid")
	// ErrTimestampOutOfTolerance indicates a possible replay. -> 401
	ErrTimestampOutOfTolerance = errors.New("webhook signature timestamp outside tolerance")

	// ErrInvalidEnvelope indicates a malformed body or missing required field. -> 400
	ErrInvalidEnvelope = errors.New("invalid webhook envelope")
	// ErrUnsupportedEvent indicates an unknown event_type. -> 400
	ErrUnsupportedEvent = errors.New("unsupported webhook event type")
	// ErrBodyTooLarge indicates the body exceeded max_body_size. -> 400
	ErrBodyTooLarge = errors.New("webhook request body too large")

	// ErrDecryptionFailed indicates the encrypted field could not be decrypted. -> 400
	ErrDecryptionFailed = errors.New("failed to decrypt webhook payload field")
	// ErrDecryptorUnavailable indicates an encrypted field was received but no private key is configured. -> 500
	ErrDecryptorUnavailable = errors.New("webhook decryptor is not configured")
)
