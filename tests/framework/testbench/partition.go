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

package testbench

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// maxPartitionKeyLen bounds the block key path segment.
const maxPartitionKeyLen = 64

type partitionCtxKey struct{}

// PartitionRouter validates and strips the leading /<block> path segment from every request,
// making the block key available to the wrapped handler via PartitionKeyFromContext. Any
// Stateful service that partitions by PartitionByBlock routes its mux through this so every
// hosted route is addressed as /<block>/rest-of-path.
func PartitionRouter(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, rest, err := splitPartition(r.URL.Path)
		if err != nil {
			http.Error(w, "testbench: "+err.Error(), http.StatusBadRequest)
			return
		}

		scoped := r.Clone(context.WithValue(r.Context(), partitionCtxKey{}, key))
		scoped.URL.Path = rest
		scoped.URL.RawPath = ""
		inner.ServeHTTP(w, scoped)
	})
}

// PartitionKeyFromContext returns the block key set by PartitionRouter.
func PartitionKeyFromContext(ctx context.Context) (string, bool) {
	key, ok := ctx.Value(partitionCtxKey{}).(string)
	return key, ok
}

// splitPartition returns the block key and the remaining route path.
func splitPartition(path string) (key, rest string, err error) {
	trimmed := strings.TrimPrefix(path, "/")
	segment, remainder, found := strings.Cut(trimmed, "/")
	if segment == "" {
		return "", "", fmt.Errorf("every route is partitioned by block, so a request needs a "+
			"leading /<block> segment; got %q", path)
	}
	if isReservedPartitionKey(segment) {
		return "", "", fmt.Errorf("%q is a route root, not a block key: this service partitions "+
			"by block, so the path is /<block>%s — a caller configured with the unpartitioned "+
			"base URL lands here", segment, path)
	}
	if len(segment) > maxPartitionKeyLen {
		return "", "", fmt.Errorf("block key %q is longer than %d characters, so it is not a block "+
			"key", segment, maxPartitionKeyLen)
	}
	for _, c := range segment {
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return "", "", fmt.Errorf("block key %q contains %q; a block key is lowercase letters, "+
				"digits and dashes only", segment, string(c))
		}
	}
	if !found {
		return segment, "/", nil
	}
	return segment, "/" + remainder, nil
}

func isReservedPartitionKey(key string) bool {
	switch key {
	case "v1", "test", "testbench":
		return true
	default:
		return false
	}
}
