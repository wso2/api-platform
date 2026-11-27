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

package helmgateway

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/helm"
)

// fakeReleases is a releaseValuesReader over a fixed set of releases in testNamespace,
// recording every name it is asked for.
type fakeReleases struct {
	values map[string]map[string]interface{}
	asked  []string
}

func (f *fakeReleases) read(namespace, name string) (map[string]interface{}, bool, error) {
	f.asked = append(f.asked, name)
	if namespace != testNamespace {
		return nil, false, nil
	}
	v, ok := f.values[name]
	return v, ok, nil
}

// legacyGateway is a gateway name long enough to have moved off its readable release
// name, and so to have a legacy one.
var legacyGateway = strings.Repeat("g", 40)

func previousInput(t *testing.T, fromGatewayAPI bool, objs ...client.Object) PreviousReleaseInput {
	t.Helper()
	return PreviousReleaseInput{
		Config:         testConfig(),
		Client:         fake.NewClientBuilder().WithScheme(testScheme(t)).WithObjects(objs...).Build(),
		GatewayName:    legacyGateway,
		Namespace:      testNamespace,
		FromGatewayAPI: fromGatewayAPI,
	}
}

func legacyValues() map[string]interface{} { return map[string]interface{}{"legacy": true} }

// A gateway with no legacy release name is looked up under its current name only: an
// empty name would make Helm fail the lookup ("Release name is invalid").
func TestPreviousReleaseValuesWithoutLegacyName(t *testing.T) {
	require.Empty(t, helm.LegacyReleaseName(testGateway))
	f := &fakeReleases{}
	in := previousInput(t, false)
	in.GatewayName = testGateway
	_, found, err := previousReleaseValues(context.Background(), f.read, in)
	require.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, []string{helm.GetReleaseName(testGateway)}, f.asked)
}

// The current release wins; the legacy one is read only when there is none.
func TestPreviousReleaseValuesPrefersCurrentRelease(t *testing.T) {
	legacy := helm.LegacyReleaseName(legacyGateway)
	require.NotEmpty(t, legacy)
	f := &fakeReleases{values: map[string]map[string]interface{}{
		helm.GetReleaseName(legacyGateway): {"current": true},
		legacy:                             legacyValues(),
	}}
	v, found, err := previousReleaseValues(context.Background(), f.read, previousInput(t, false))
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, map[string]interface{}{"current": true}, v)

	delete(f.values, helm.GetReleaseName(legacyGateway))
	v, found, err = previousReleaseValues(context.Background(), f.read, previousInput(t, false))
	require.NoError(t, err)
	assert.True(t, found, "an uncontested legacy release is this gateway's")
	assert.Equal(t, legacyValues(), v)
}

// When a gateway of the other kind claims the same namespace and name, neither the
// current nor the legacy release is this gateway's: the lookup fails, for either kind,
// before reading any release, so their values cannot decide this gateway's admin
// credential.
func TestPreviousReleaseValuesFailsOnContestedName(t *testing.T) {
	legacy := helm.LegacyReleaseName(legacyGateway)
	releases := map[string]map[string]map[string]interface{}{
		"current release": {helm.GetReleaseName(legacyGateway): {"current": true}},
		"legacy release":  {legacy: legacyValues()},
	}
	for name, values := range releases {
		t.Run(name, func(t *testing.T) {
			apiGW := apiGateway()
			apiGW.Name = legacyGateway
			f := &fakeReleases{values: values}
			_, found, err := previousReleaseValues(context.Background(), f.read, previousInput(t, true, apiGW))
			require.Error(t, err, "a Gateway API Gateway, with an APIGateway of the same name")
			assert.Contains(t, err.Error(), "an APIGateway")
			assert.False(t, found)
			assert.Empty(t, f.asked, "no release is read for a contested name")

			k8sGW := k8sGateway(managedClass)
			k8sGW.Name = legacyGateway
			f = &fakeReleases{values: values}
			_, found, err = previousReleaseValues(context.Background(), f.read, previousInput(t, false, k8sGW))
			require.Error(t, err, "an APIGateway, with a managed Gateway API Gateway of the same name")
			assert.Contains(t, err.Error(), "a Kubernetes Gateway")
			assert.False(t, found)
			assert.Empty(t, f.asked)

			// A Gateway on a class this operator does not manage claims nothing.
			foreign := k8sGateway("someone-else")
			foreign.Name = legacyGateway
			f = &fakeReleases{values: values}
			_, found, err = previousReleaseValues(context.Background(), f.read, previousInput(t, false, foreign))
			require.NoError(t, err)
			assert.True(t, found)
		})
	}
}
