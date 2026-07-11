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

package management

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOperation_EffectiveMatch(t *testing.T) {
	get := OperationMethod("GET")
	prefix := OperationPathMatchType("PathPrefix")

	t.Run("simple form uses top-level method/path, no type/headers", func(t *testing.T) {
		op := Operation{Method: &get, Path: Ptr("/hot")}
		assert.Equal(t, "GET", op.EffectiveMethod())
		assert.Equal(t, "/hot", op.EffectivePath())
		assert.Equal(t, "", op.EffectivePathMatchType(), "simple form carries no explicit path type")
		assert.Nil(t, op.EffectiveHeaders())
	})

	t.Run("match form supplies method/path/type/headers", func(t *testing.T) {
		op := Operation{
			Match: &OperationMatch{
				Method:  "GET",
				Path:    OperationPathMatch{Value: "/via-match", Type: &prefix},
				Headers: &[]OperationHeaderMatch{{Name: "x-variant", Value: "alpha"}},
			},
		}
		assert.Equal(t, "GET", op.EffectiveMethod())
		assert.Equal(t, "/via-match", op.EffectivePath())
		assert.Equal(t, "PathPrefix", op.EffectivePathMatchType())
		assert.Len(t, op.EffectiveHeaders(), 1)
	})

	t.Run("match omitting type defaults to Exact", func(t *testing.T) {
		op := Operation{Match: &OperationMatch{Method: "GET", Path: OperationPathMatch{Value: "/x"}}}
		assert.Equal(t, "Exact", op.EffectivePathMatchType())
	})
}
