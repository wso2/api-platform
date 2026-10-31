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

package managementportal

import (
	"context"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/core/util/unique"
	"github.com/wso2/api-platform/tests/framework/suites/ui/steps/uivalue"
)

func TestUnknownActorsAreRejectedBeforeAnyRequest(t *testing.T) {
	steps := &Steps{}
	for _, who := range []string{"eve", "Admin", "org b admin"} {
		_, err := steps.bearer(context.Background(), who)
		require.ErrorContains(t, err, "unknown control-plane actor", who)
	}
	require.ErrorContains(t, steps.signInAs(context.Background(), "admin"), "unknown Management Portal user")
}

func TestSecondOrganizationKindsAreDistinctAndOrderedLikeTheirResources(t *testing.T) {
	require.NotEqual(t, cleanup.KindAPI.Name, KindSecondOrganizationRESTAPI.Name)
	require.NotEqual(t, cleanup.KindGateway.Name, KindSecondOrganizationGateway.Name)
	require.Equal(t, cleanup.KindAPI.Order, KindSecondOrganizationRESTAPI.Order)
	require.Equal(t, cleanup.KindGateway.Order, KindSecondOrganizationGateway.Order)
	require.Less(t, KindSecondOrganizationRESTAPI.Order, KindSecondOrganizationGateway.Order,
		"an API is removed before the gateway it could be deployed to")
}

func TestControlPlaneTLSTrustsThePlatformAPICertificate(t *testing.T) {
	config := controlPlaneTLS()
	require.Equal(t, "platform-api", config.ServerName)
	require.NotNil(t, config.RootCAs)
}

func TestGeneratedHandlesAreControlPlaneHandlesAndExpandAsContextValues(t *testing.T) {
	generator, err := unique.NewGenerator()
	require.NoError(t, err)
	ctx := tcontext.WithLocal(
		tcontext.WithShared(context.Background(), tcontext.NewShared("ui")),
		tcontext.NewLocal("runner"),
	)
	require.NoError(t, unique.Install(ctx, generator))

	require.NoError(t, generateHandle(ctx, "Tenant_UI gateway", "gateway"))
	require.NoError(t, generateHandle(ctx, "Tenant_UI gateway", "other"))
	first, err := uivalue.Expand(ctx, "${CTX:gateway}")
	require.NoError(t, err)
	second, err := uivalue.Expand(ctx, "${CTX:other}")
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^[a-z0-9-]+$`), first)
	require.NotEqual(t, first, second, "each generated handle is unique")

	require.ErrorContains(t, generateHandle(context.Background(), "x", "k"), "without runner context")
}
