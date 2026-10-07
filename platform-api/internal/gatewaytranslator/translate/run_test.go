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

package translate

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wso2/api-platform/platform-api/internal/constants"
	"github.com/wso2/api-platform/platform-api/internal/gatewaytranslator/gwversion"
)

// fakeArtifact records what ran, in order, and carries the apiVersion the swap acts on.
type fakeArtifact struct {
	apiVersion string
	trail      []string
}

func (f *fakeArtifact) GetApiVersion() string  { return f.apiVersion }
func (f *fakeArtifact) SetApiVersion(v string) { f.apiVersion = v }

func newFake() *fakeArtifact { return &fakeArtifact{apiVersion: constants.GatewayApiVersion} }

func recordingStep(below, name string) Step {
	return Step{Below: below, Name: name, Apply: func(a any, r *Report) error {
		f := a.(*fakeArtifact)
		f.trail = append(f.trail, name)
		r.Warn("Fake", "spec."+name, "ran")
		return nil
	}}
}

func kindWith(steps ...Step) Kind {
	return Kind{
		GatewayKind: "Fake",
		Normalize: func(_ string, a any) error {
			a.(*fakeArtifact).trail = append(a.(*fakeArtifact).trail, "normalize")
			return nil
		},
		Steps: steps,
	}
}

func TestRun_AppliesOnlyStepsBelowTheGatewayVersion_InOrder(t *testing.T) {
	k := kindWith(
		recordingStep(gwversion.MinGatewayV1Version, "below-1.2.0"),
		recordingStep(gwversion.MinMCPSpecVersionListGatewayVersion, "below-2026.09.24"),
		recordingStep(gwversion.MinGatewayV1Version, "also-below-1.2.0"),
	)

	t.Run("gateway 1.1.0 runs every step and swaps apiVersion", func(t *testing.T) {
		a := newFake()
		rep, err := Run(k, "1.0", "1.1.0", a)
		require.NoError(t, err)
		assert.Equal(t, []string{"normalize", "below-1.2.0", "below-2026.09.24", "also-below-1.2.0"}, a.trail)
		assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.apiVersion)
		assert.Len(t, rep.Warnings(), 3)
	})

	t.Run("gateway 1.2.0 runs only the CalVer-gated step and keeps v1", func(t *testing.T) {
		a := newFake()
		rep, err := Run(k, "1.0", "1.2.0", a)
		require.NoError(t, err)
		assert.Equal(t, []string{"normalize", "below-2026.09.24"}, a.trail)
		assert.Equal(t, constants.GatewayApiVersion, a.apiVersion)
		assert.Len(t, rep.Warnings(), 1)
	})

	t.Run("gateway 2026.09.24 only normalizes", func(t *testing.T) {
		a := newFake()
		rep, err := Run(k, "1.0", "2026.09.24", a)
		require.NoError(t, err)
		assert.Equal(t, []string{"normalize"}, a.trail)
		assert.Equal(t, constants.GatewayApiVersion, a.apiVersion)
		assert.True(t, rep.Empty())
	})

	for _, raw := range []string{"", "it-e2e"} {
		t.Run("gateway "+raw+" is a current build: normalize only", func(t *testing.T) {
			a := newFake()
			rep, err := Run(k, "1.0", raw, a)
			require.NoError(t, err)
			assert.Equal(t, []string{"normalize"}, a.trail)
			assert.Equal(t, constants.GatewayApiVersion, a.apiVersion)
			assert.True(t, rep.Empty())
		})
	}
}

// The apiVersion swap belongs to Run, so a kind with no steps still gets it.
func TestRun_SwapsApiVersionForEveryKindBelowV1(t *testing.T) {
	k := Kind{GatewayKind: "Bare"}
	a := newFake()
	_, err := Run(k, "1.0", "1.1.0", a)
	require.NoError(t, err)
	assert.Equal(t, constants.GatewayApiVersionV1Alpha1, a.apiVersion)
	assert.Empty(t, a.trail, "a nil Normalize is the identity")
}

func TestRun_NormalizeErrorStopsTheRun(t *testing.T) {
	boom := errors.New("wrong type")
	k := Kind{
		GatewayKind: "Fake",
		Normalize:   func(string, any) error { return boom },
		Steps:       []Step{recordingStep(gwversion.MinGatewayV1Version, "never")},
	}
	a := newFake()
	_, err := Run(k, "1.0", "1.1.0", a)
	require.ErrorIs(t, err, boom)
	assert.Empty(t, a.trail)
	assert.Equal(t, constants.GatewayApiVersion, a.apiVersion, "no swap after a failed normalize")
}

func TestRun_StepErrorNamesTheStep(t *testing.T) {
	k := Kind{GatewayKind: "Fake", Steps: []Step{{
		Below: gwversion.MinGatewayV1Version,
		Name:  "explode",
		Apply: func(any, *Report) error { return errors.New("kaboom") },
	}}}
	_, err := Run(k, "1.0", "1.0.0", newFake())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Fake")
	assert.Contains(t, err.Error(), "explode")
}

func TestRun_ArtifactWithoutApiVersionAccessors(t *testing.T) {
	k := Kind{GatewayKind: "Bare"}
	type plain struct{ Foo string }

	_, err := Run(k, "1.0", "1.1.0", &plain{})
	assert.Error(t, err, "an old gateway needs the swap, which needs the accessors")

	_, err = Run(k, "1.0", "1.2.0", &plain{})
	assert.NoError(t, err, "a current gateway needs no swap")
}
