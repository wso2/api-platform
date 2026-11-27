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

package helm

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v3/pkg/chart"
)

func chartWithFeatures(annotation *string) *chart.Chart {
	md := &chart.Metadata{Name: "gateway", Version: "2026.11.19-r1"}
	if annotation != nil {
		md.Annotations = map[string]string{ChartFeaturesAnnotation: *annotation}
	}
	return &chart.Chart{Metadata: md}
}

func TestCheckChartFeatures(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name       string
		annotation *string
		required   []string
		wantErr    bool
	}{
		{"nothing required, no annotation", nil, nil, false},
		{"declared", str(FeatureBasicAuthPasswordSecretRef), []string{FeatureBasicAuthPasswordSecretRef}, false},
		{"declared among others with spaces", str(" other , " + FeatureBasicAuthPasswordSecretRef + " ,"), []string{FeatureBasicAuthPasswordSecretRef}, false},
		{"older chart without the annotation", nil, []string{FeatureBasicAuthPasswordSecretRef}, true},
		{"annotation without the feature", str("other"), []string{FeatureBasicAuthPasswordSecretRef}, true},
		{"empty annotation", str(""), []string{FeatureBasicAuthPasswordSecretRef}, true},
		{"prefix of a feature does not match", str("basicAuthPasswordSecretRefV0"), []string{FeatureBasicAuthPasswordSecretRef}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkChartFeatures(chartWithFeatures(tc.annotation), tc.required)
			if !tc.wantErr {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, ErrChartFeatureUnsupported))
			assert.Contains(t, err.Error(), "2026.11.19-r1", "the error names the chart version in use")
			assert.Contains(t, err.Error(), FeatureBasicAuthPasswordSecretRef)
		})
	}
	assert.ErrorIs(t, checkChartFeatures(&chart.Chart{}, []string{FeatureBasicAuthPasswordSecretRef}), ErrChartFeatureUnsupported,
		"a chart without metadata supports nothing")
}
