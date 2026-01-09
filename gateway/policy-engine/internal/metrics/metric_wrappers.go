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
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"
)

// Enabled indicates whether metrics collection is enabled.
// This is set once at startup via SetEnabled() and should not be modified after.
var Enabled bool

// metricsInitialized tracks whether metrics have been initialized
var metricsInitialized atomic.Bool

// Counter wraps prometheus.Counter with a noop implementation when disabled
type Counter interface {
	Inc()
	Add(float64)
}

// CounterVec wraps prometheus.CounterVec with a noop implementation when disabled
type CounterVec interface {
	WithLabelValues(labels ...string) Counter
	With(prometheus.Labels) Counter
}

// Histogram wraps prometheus.Histogram with a noop implementation when disabled
type Histogram interface {
	Observe(float64)
}

// HistogramVec wraps prometheus.HistogramVec with a noop implementation when disabled
type HistogramVec interface {
	WithLabelValues(labels ...string) Histogram
	With(prometheus.Labels) Histogram
}

// Gauge wraps prometheus.Gauge with a noop implementation when disabled
type Gauge interface {
	Set(float64)
	Inc()
	Dec()
	Add(float64)
	Sub(float64)
}

// GaugeVec wraps prometheus.GaugeVec with a noop implementation when disabled
type GaugeVec interface {
	WithLabelValues(labels ...string) Gauge
	With(prometheus.Labels) Gauge
}

// GaugeFunc wraps prometheus.GaugeFunc for callback-based gauges
type GaugeFunc interface {
	prometheus.Metric
	prometheus.Collector
}

// Noop implementations - these are ALWAYS safe to call methods on (never nil)

// noopCounter is a no-operation counter that does nothing when metrics are disabled
type noopCounter struct{}

func (noopCounter) Inc()        {}
func (noopCounter) Add(float64) {}

// noopCounterVec is a no-operation counter vector that returns noop counters
type noopCounterVec struct{}

func (noopCounterVec) WithLabelValues(...string) Counter { return safeNoopCounter }
func (noopCounterVec) With(prometheus.Labels) Counter    { return safeNoopCounter }

// noopHistogram is a no-operation histogram that does nothing when metrics are disabled
type noopHistogram struct{}

func (noopHistogram) Observe(float64) {}

// noopHistogramVec is a no-operation histogram vector that returns noop histograms
type noopHistogramVec struct{}

func (noopHistogramVec) WithLabelValues(...string) Histogram { return safeNoopHistogram }
func (noopHistogramVec) With(prometheus.Labels) Histogram    { return safeNoopHistogram }

// noopGauge is a no-operation gauge that does nothing when metrics are disabled
type noopGauge struct{}

func (noopGauge) Set(float64) {}
func (noopGauge) Inc()        {}
func (noopGauge) Dec()        {}
func (noopGauge) Add(float64) {}
func (noopGauge) Sub(float64) {}

// noopGaugeVec is a no-operation gauge vector that returns noop gauges
type noopGaugeVec struct{}

func (noopGaugeVec) WithLabelValues(...string) Gauge { return safeNoopGauge }
func (noopGaugeVec) With(prometheus.Labels) Gauge    { return safeNoopGauge }

// safeNoopGaugeFunc returns a singleton noop GaugeFunc that's safe to use
func safeNoopGaugeFunc() GaugeFunc {
	return nil // Return nil - registration function will skip it
}

// Safe singleton instances - these are ALWAYS safe to return from factory functions
var (
	safeNoopCounter   Counter    = noopCounter{}
	safeNoopHistogram Histogram  = noopHistogram{}
	safeNoopGauge    Gauge      = noopGauge{}
)

// Wrapper types to adapt prometheus types to our interfaces

// counterVecWrapper wraps prometheus.CounterVec to implement CounterVec interface
type counterVecWrapper struct {
	*prometheus.CounterVec
}

func (c *counterVecWrapper) WithLabelValues(labels ...string) Counter {
	return c.CounterVec.WithLabelValues(labels...)
}

func (c *counterVecWrapper) With(labels prometheus.Labels) Counter {
	return c.CounterVec.With(labels)
}

// histogramVecWrapper wraps prometheus.HistogramVec to implement HistogramVec interface
type histogramVecWrapper struct {
	*prometheus.HistogramVec
}

func (h *histogramVecWrapper) WithLabelValues(labels ...string) Histogram {
	return h.HistogramVec.WithLabelValues(labels...)
}

func (h *histogramVecWrapper) With(labels prometheus.Labels) Histogram {
	return h.HistogramVec.With(labels)
}

// gaugeVecWrapper wraps prometheus.GaugeVec to implement GaugeVec interface
type gaugeVecWrapper struct {
	*prometheus.GaugeVec
}

func (g *gaugeVecWrapper) WithLabelValues(labels ...string) Gauge {
	return g.GaugeVec.WithLabelValues(labels...)
}

func (g *gaugeVecWrapper) With(labels prometheus.Labels) Gauge {
	return g.GaugeVec.With(labels)
}

// IsEnabled returns whether metrics collection is enabled
func IsEnabled() bool {
	return Enabled
}

// SetEnabled sets whether metrics collection is enabled.
// This must be called before Init() for proper effect.
func SetEnabled(e bool) {
	Enabled = e
}

// newCounterVec creates a new CounterVec that is safe to use even when disabled
func newCounterVec(opts prometheus.CounterOpts, labelNames []string) CounterVec {
	if Enabled {
		return &counterVecWrapper{prometheus.NewCounterVec(opts, labelNames)}
	}
	// Return properly initialized noop instance (not zero value!)
	return noopCounterVec{}
}

// newCounter creates a new Counter that is safe to use even when disabled
func newCounter(opts prometheus.CounterOpts) Counter {
	if Enabled {
		return prometheus.NewCounter(opts)
	}
	return safeNoopCounter
}

// newHistogramVec creates a new HistogramVec that is safe to use even when disabled
func newHistogramVec(opts prometheus.HistogramOpts, labelNames []string) HistogramVec {
	if Enabled {
		return &histogramVecWrapper{prometheus.NewHistogramVec(opts, labelNames)}
	}
	// Return properly initialized noop instance (not zero value!)
	return noopHistogramVec{}
}

// newHistogram creates a new Histogram that is safe to use even when disabled
func newHistogram(opts prometheus.HistogramOpts) Histogram {
	if Enabled {
		return prometheus.NewHistogram(opts)
	}
	return safeNoopHistogram
}

// newGaugeVec creates a new GaugeVec that is safe to use even when disabled
func newGaugeVec(opts prometheus.GaugeOpts, labelNames []string) GaugeVec {
	if Enabled {
		return &gaugeVecWrapper{prometheus.NewGaugeVec(opts, labelNames)}
	}
	// Return properly initialized noop instance (not zero value!)
	return noopGaugeVec{}
}

// newGauge creates a new Gauge that is safe to use even when disabled
func newGauge(opts prometheus.GaugeOpts) Gauge {
	if Enabled {
		return prometheus.NewGauge(opts)
	}
	return safeNoopGauge
}

// newGaugeFunc creates a new GaugeFunc that is safe to use even when disabled
func newGaugeFunc(opts prometheus.GaugeOpts, f func() float64) GaugeFunc {
	if Enabled {
		return prometheus.NewGaugeFunc(opts, f)
	}
	// Return a noop instance - registration function will skip it if nil
	return safeNoopGaugeFunc()
}
