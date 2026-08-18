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

package publishers

import (
	"context"
	"fmt"
	"time"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

// Sink names, matching the config.TrafficLogSink* constants. Duplicated as local
// constants so they can be used as metric labels without importing config into
// every call site.
const (
	sinkNameStdout = config.TrafficLogSinkStdout
	sinkNameFile   = config.TrafficLogSinkFile
	sinkNameHTTP   = config.TrafficLogSinkHTTP
)

// Reasons recorded on policy_engine_traffic_log_dropped_total.
const (
	// dropReasonQueueFull: the HTTP sink's bounded queue had no room.
	dropReasonQueueFull = "queue_full"
	// dropReasonSendFailed: the HTTP sink exhausted its retries.
	dropReasonSendFailed = "send_failed"
	// dropReasonWriteFailed: a local write returned an error.
	dropReasonWriteFailed = "write_failed"
	// dropReasonRotateFailed: the file sink could not rotate, so the line that
	// triggered the rotation was not written.
	dropReasonRotateFailed = "rotate_failed"
	// dropReasonBackpressure: the HTTP sink abandoned a batch's remaining retries
	// because the queue was filling behind it. Distinct from send_failed so an
	// operator can tell "the receiver is slow" from "the receiver is broken".
	dropReasonBackpressure = "backpressure"
)

// Codes recorded on policy_engine_traffic_log_write_errors_total for non-HTTP
// failures. HTTP failures use the numeric status code instead.
const (
	errCodeWrite     = "write"
	errCodeRotate    = "rotate"
	errCodeTransport = "transport"
)

// sinkCleanupTimeout bounds the rollback close when one sink in the set fails to
// build. This is startup, not shutdown: nothing has been written yet, so the
// close should be near-instant and this exists only so a wedged sink cannot mask
// the construction error the operator actually needs to see.
const sinkCleanupTimeout = 5 * time.Second

// newSinks builds the sink set named by traffic_logging.outputs.
//
// It fails closed: any sink that cannot be constructed returns an error, and the
// caller must refuse to start rather than continue with a partial set. Silently
// dropping an unbuildable file or http sink would leave the operator on stdout —
// putting request and response bodies back into the container log, and therefore
// onto the node's disk and into any node-level collector, which is the exact
// disclosure those sinks are configured to prevent.
//
// Sinks already built are closed before the error is returned, so a failure part
// way through the list does not leak a file descriptor or an orphaned goroutine.
func newSinks(cfg *config.TrafficLoggingConfig) ([]Sink, error) {
	outputs, err := config.NormalizeTrafficLogOutputs(cfg.Outputs)
	if err != nil {
		return nil, err
	}

	sinks := make([]Sink, 0, len(outputs))
	closeAll := func() {
		// Bounded, and deliberately shared across every sink so the whole cleanup
		// is capped rather than each sink getting its own budget. httpSink.Close
		// waits for its sender goroutine; an unbounded context here would let a
		// failed startup hang instead of reporting the error that caused it.
		ctx, cancel := context.WithTimeout(context.Background(), sinkCleanupTimeout)
		defer cancel()
		for _, s := range sinks {
			_ = s.Close(ctx)
		}
	}

	for _, name := range outputs {
		switch name {
		case sinkNameStdout:
			sinks = append(sinks, newStdoutSink())
		case sinkNameFile:
			s, err := newFileSink(cfg.File)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("traffic_logging.file: %w", err)
			}
			sinks = append(sinks, s)
		case sinkNameHTTP:
			s, err := newHTTPSink(cfg.HTTP)
			if err != nil {
				closeAll()
				return nil, fmt.Errorf("traffic_logging.http: %w", err)
			}
			sinks = append(sinks, s)
		default:
			// Unreachable: NormalizeTrafficLogOutputs rejects unknown names.
			closeAll()
			return nil, fmt.Errorf("unsupported traffic_logging.outputs entry %q", name)
		}
		initSinkMetrics(name)
	}
	return sinks, nil
}

// sinkFailureLabels lists the drop reasons and error codes each sink can
// actually produce. Anything a sink cannot emit is deliberately absent, so the
// scrape does not advertise a failure mode that will never occur for it.
var sinkFailureLabels = map[string]struct {
	dropReasons []string
	errCodes    []string
}{
	sinkNameStdout: {
		dropReasons: []string{dropReasonWriteFailed},
		errCodes:    []string{errCodeWrite},
	},
	sinkNameFile: {
		dropReasons: []string{dropReasonWriteFailed, dropReasonRotateFailed},
		errCodes:    []string{errCodeWrite, errCodeRotate},
	},
	sinkNameHTTP: {
		dropReasons: []string{dropReasonQueueFull, dropReasonSendFailed, dropReasonBackpressure},
		// Only the transport code is pre-created. The HTTP sink also labels by
		// response status, and those are unbounded — materializing every
		// possible status would be worse than the gap it closes.
		errCodes: []string{errCodeTransport},
	},
}

// initSinkMetrics materializes a configured sink's counters at zero.
//
// A Prometheus counter with labels does not exist in the scrape until it is
// first incremented, so on a healthy gateway traffic_log_dropped_total is
// simply absent. That makes a dashboard panel read "No data" rather than 0, and
// leaves an operator unable to tell "nothing was dropped" apart from "the
// metrics path is broken" — an unacceptable ambiguity for the one series that
// makes silent traffic-log loss visible. Creating the series up front costs a
// handful of samples and removes the ambiguity.
func initSinkMetrics(sink string) {
	// No nil guard needed here: every m* helper is guarded, so this is safe
	// before metrics.Init() and consistent with the write paths.
	mWritten(sink, 0)
	labels, ok := sinkFailureLabels[sink]
	if !ok {
		return
	}
	for _, reason := range labels.dropReasons {
		mDropped(sink, reason, 0)
	}
	for _, code := range labels.errCodes {
		mWriteError(sink, code, 0)
	}
}
