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

package handler

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"platform-api/src/config"

	"github.com/gin-gonic/gin"
)

func TestGatewayInternalAPIHandler_rejectIfReadOnly(t *testing.T) {
	const (
		orgA = "11111111-1111-1111-1111-111111111111"
		orgB = "22222222-2222-2222-2222-222222222222"
	)
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	// The constructor never dereferences the services, so nil is fine here.
	h := NewGatewayInternalAPIHandler(nil, nil, &config.ReadOnly{Organizations: []string{orgA}}, logger)

	newCtx := func() (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/internal/v1/gateways/gw/manifest", nil)
		return c, w
	}

	c, w := newCtx()
	if !h.rejectIfReadOnly(c, orgA, "gw") {
		t.Fatal("expected write for read-only organization to be rejected")
	}
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	c, w = newCtx()
	if h.rejectIfReadOnly(c, orgB, "gw") {
		t.Fatal("expected write for non-read-only organization to be allowed")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want the recorder left untouched (200)", w.Code)
	}
}
