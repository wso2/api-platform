/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// SetEnabled / IsEnabled Tests
// =============================================================================

func TestSetEnabled_True(t *testing.T) {
	// Save original state
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)
	assert.True(t, IsEnabled())
}

func TestSetEnabled_False(t *testing.T) {
	// Save original state
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)
	assert.False(t, IsEnabled())
}

// =============================================================================
// Noop Implementation Tests (when disabled)
// =============================================================================

func TestNoopCounter(t *testing.T) {
	counter := noopCounter{}

	// These should not panic
	counter.Inc()
	counter.Add(5.0)
}

func TestNoopCounterVec(t *testing.T) {
	vec := noopCounterVec{}

	counter := vec.WithLabelValues("label1", "label2")
	assert.NotNil(t, counter)

	counter.Inc()
	counter.Add(10.0)

	counterWithLabels := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, counterWithLabels)
}

func TestNoopHistogram(t *testing.T) {
	histogram := noopHistogram{}

	// Should not panic
	histogram.Observe(1.5)
	histogram.Observe(0.0)
	histogram.Observe(100.0)
}

func TestNoopHistogramVec(t *testing.T) {
	vec := noopHistogramVec{}

	histogram := vec.WithLabelValues("label1")
	assert.NotNil(t, histogram)

	histogram.Observe(2.5)

	histogramWithLabels := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, histogramWithLabels)
}

func TestNoopGauge(t *testing.T) {
	gauge := noopGauge{}

	// These should not panic
	gauge.Set(10.0)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(5.0)
	gauge.Sub(3.0)
}

func TestNoopGaugeVec(t *testing.T) {
	vec := noopGaugeVec{}

	gauge := vec.WithLabelValues("label1", "label2")
	assert.NotNil(t, gauge)

	gauge.Set(100.0)
	gauge.Inc()

	gaugeWithLabels := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, gaugeWithLabels)
}

// =============================================================================
// Factory Function Tests (Disabled Mode)
// =============================================================================

func TestNewCounterVec_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	vec := newCounterVec(prometheus.CounterOpts{
		Name: "test_counter",
		Help: "Test counter",
	}, []string{"label"})

	assert.NotNil(t, vec)

	counter := vec.WithLabelValues("value")
	assert.NotNil(t, counter)

	// Should not panic
	counter.Inc()
	counter.Add(1.0)
}

func TestNewCounter_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	counter := newCounter(prometheus.CounterOpts{
		Name: "test_counter_single",
		Help: "Test counter single",
	})

	assert.NotNil(t, counter)

	// Should not panic
	counter.Inc()
	counter.Add(1.0)
}

func TestNewHistogramVec_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	vec := newHistogramVec(prometheus.HistogramOpts{
		Name:    "test_histogram",
		Help:    "Test histogram",
		Buckets: []float64{0.1, 0.5, 1.0},
	}, []string{"label"})

	assert.NotNil(t, vec)

	histogram := vec.WithLabelValues("value")
	assert.NotNil(t, histogram)

	// Should not panic
	histogram.Observe(0.5)
}

func TestNewHistogram_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	histogram := newHistogram(prometheus.HistogramOpts{
		Name:    "test_histogram_single",
		Help:    "Test histogram single",
		Buckets: []float64{0.1, 0.5, 1.0},
	})

	assert.NotNil(t, histogram)

	// Should not panic
	histogram.Observe(0.5)
}

func TestNewGaugeVec_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	vec := newGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge",
		Help: "Test gauge",
	}, []string{"label"})

	assert.NotNil(t, vec)

	gauge := vec.WithLabelValues("value")
	assert.NotNil(t, gauge)

	// Should not panic
	gauge.Set(10.0)
	gauge.Inc()
	gauge.Dec()
}

func TestNewGauge_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	gauge := newGauge(prometheus.GaugeOpts{
		Name: "test_gauge_single",
		Help: "Test gauge single",
	})

	assert.NotNil(t, gauge)

	// Should not panic
	gauge.Set(10.0)
	gauge.Inc()
	gauge.Dec()
	gauge.Add(5.0)
	gauge.Sub(3.0)
}

func TestNewGaugeFunc_Disabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(false)

	gaugeFunc := newGaugeFunc(prometheus.GaugeOpts{
		Name: "test_gauge_func",
		Help: "Test gauge func",
	}, func() float64 {
		return 42.0
	})

	// When disabled, returns nil (which is acceptable)
	assert.Nil(t, gaugeFunc)
}

// =============================================================================
// Factory Function Tests (Enabled Mode)
// =============================================================================

func TestNewCounterVec_Enabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newCounterVec(prometheus.CounterOpts{
		Name: "test_counter_enabled",
		Help: "Test counter enabled",
	}, []string{"label"})

	assert.NotNil(t, vec)

	counter := vec.WithLabelValues("value")
	assert.NotNil(t, counter)

	counter.Inc()
	counter.Add(1.0)
}

func TestNewGaugeVec_Enabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newGaugeVec(prometheus.GaugeOpts{
		Name: "test_gauge_enabled",
		Help: "Test gauge enabled",
	}, []string{"label"})

	assert.NotNil(t, vec)

	gauge := vec.WithLabelValues("value")
	assert.NotNil(t, gauge)

	gauge.Set(10.0)
}

func TestNewHistogramVec_Enabled(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newHistogramVec(prometheus.HistogramOpts{
		Name:    "test_histogram_enabled",
		Help:    "Test histogram enabled",
		Buckets: []float64{0.1, 0.5, 1.0},
	}, []string{"label"})

	assert.NotNil(t, vec)

	histogram := vec.WithLabelValues("value")
	assert.NotNil(t, histogram)

	histogram.Observe(0.5)
}

// =============================================================================
// Safe Singleton Tests
// =============================================================================

func TestSafeNoopCounter_NotNil(t *testing.T) {
	assert.NotNil(t, safeNoopCounter)
}

func TestSafeNoopHistogram_NotNil(t *testing.T) {
	assert.NotNil(t, safeNoopHistogram)
}

func TestSafeNoopGauge_NotNil(t *testing.T) {
	assert.NotNil(t, safeNoopGauge)
}

// =============================================================================
// Wrapper Tests
// =============================================================================

func TestCounterVecWrapper_With(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newCounterVec(prometheus.CounterOpts{
		Name: "test_wrapper_counter",
		Help: "Test wrapper counter",
	}, []string{"key"})

	counter := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, counter)
}

func TestHistogramVecWrapper_With(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newHistogramVec(prometheus.HistogramOpts{
		Name:    "test_wrapper_histogram",
		Help:    "Test wrapper histogram",
		Buckets: []float64{0.1, 1.0},
	}, []string{"key"})

	histogram := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, histogram)
}

func TestGaugeVecWrapper_With(t *testing.T) {
	original := Enabled
	defer func() { Enabled = original }()

	SetEnabled(true)

	vec := newGaugeVec(prometheus.GaugeOpts{
		Name: "test_wrapper_gauge",
		Help: "Test wrapper gauge",
	}, []string{"key"})

	gauge := vec.With(prometheus.Labels{"key": "value"})
	assert.NotNil(t, gauge)
}
