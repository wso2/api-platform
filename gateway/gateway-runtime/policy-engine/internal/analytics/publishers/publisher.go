/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package publishers

import (
	"context"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/analytics/dto"
)

// Publisher represents an analytics publisher.
type Publisher interface {
	Publish(event *dto.Event)
}

// Closer is implemented by publishers that hold resources or buffer events and
// therefore need to be shut down cleanly. It is kept separate from Publisher so a
// publisher with nothing to release does not have to carry a no-op Close.
//
// Analytics.Close type-asserts each publisher against this interface during
// graceful shutdown. Without it, a buffering publisher loses its in-flight batch on
// every pod restart, rolling update and scale-down.
type Closer interface {
	// Close flushes any buffered events and releases resources. It must be
	// idempotent and must respect ctx's deadline.
	Close(ctx context.Context) error
}
