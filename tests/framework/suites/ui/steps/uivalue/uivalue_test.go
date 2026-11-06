/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.  You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package uivalue

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
)

func TestExpandUIValueKeepsScenarioValuesStable(t *testing.T) {
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("ui")),
		tcontext.NewLocal("scenario"),
	)
	require.NoError(t, unique.Install(ctx, generator))
	ctx = WithExpansionState(ctx)

	first, err := Expand(ctx, "${UNIQUE:Provider} / ${UNIQUE:Template}")
	require.NoError(t, err)
	second, err := Expand(ctx, "${UNIQUE:Provider}")
	require.NoError(t, err)
	parts := strings.Split(first, " / ")
	require.Len(t, parts, 2)
	require.Equal(t, parts[0], second)
}

func TestExpandUIValueRejectsMalformedPlaceholderAfterExpansion(t *testing.T) {
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("ui")),
		tcontext.NewLocal("scenario"),
	)
	require.NoError(t, unique.Install(ctx, generator))
	ctx = WithExpansionState(ctx)

	_, err = Expand(ctx, "prefix-${UNIQUE:Provider}-${UNIQUE:broken")
	require.Error(t, err)
	require.Contains(t, err.Error(), "malformed UNIQUE placeholder")
}

func TestExpandKeepsUniqueNamesStableAlongsideContextValues(t *testing.T) {
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	local := tcontext.NewLocal("scenario")
	ctx := tcontext.WithLocal(tcontext.WithShared(context.Background(), tcontext.NewShared("ui")), local)
	require.NoError(t, unique.Install(ctx, generator))
	ctx = WithExpansionState(ctx)
	local.Set("org", "it-second-org")

	first, err := Expand(ctx, "/organizations/${CTX:org}/apis/${UNIQUE:Api}")
	require.NoError(t, err)
	second, err := Expand(ctx, "/organizations/${CTX:org}/apis/${UNIQUE:Api}")
	require.NoError(t, err)
	alone, err := Expand(ctx, "${UNIQUE:Api}")
	require.NoError(t, err)
	require.Equal(t, first, second, "a mixed argument names the same resource every time")
	require.Equal(t, "/organizations/it-second-org/apis/"+alone, first)
}
