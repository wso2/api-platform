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

package health

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSafeReason(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "reason", err: Reason("database unreachable"), want: "database unreachable"},
		{name: "wrapped reason", err: fmt.Errorf("ping: %w", Reason("database unreachable")), want: "database unreachable"},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: "timed out after 2s"},
		{name: "wrapped deadline exceeded", err: fmt.Errorf("ping: %w", context.DeadlineExceeded), want: "timed out after 2s"},
		{name: "any other error", err: errors.New("password authentication failed for user admin"), want: genericFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, safeReason(tt.err, 2*time.Second))
		})
	}
}

func TestReasonError(t *testing.T) {
	assert.Equal(t, "database unreachable", Reason("database unreachable").Error())
}
