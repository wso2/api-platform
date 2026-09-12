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

package logcapture

import (
	"fmt"
	"os"
	"sync/atomic"

	"github.com/testcontainers/testcontainers-go"
)

// bufferedLines bounds how many not-yet-written lines a Writer holds. A container
// producing faster than the writer goroutine can drain is throttled by dropping,
// never by blocking the container's own log-producer goroutine.
const bufferedLines = 1000

// Writer streams many containers' log lines into one combined file, prefixed per
// component the way `docker compose up` interleaves its services' output. Every
// Consumer's Accept is a non-blocking send: a full buffer drops the line and counts
// it, rather than ever blocking the caller.
//
// lines is never closed: a container's own log-producer goroutine (owned by
// testcontainers, not this package) keeps calling Accept for as long as the container
// exists, independent of when the block decides to Close its Writer — closing the
// channel itself would race that producer and panic on a send to a closed channel.
// Close instead signals stop and lets run drain whatever already arrived.
type Writer struct {
	lines   chan logLine
	dropped atomic.Int64
	stop    chan struct{}
	done    chan struct{}
}

type logLine struct {
	component string
	text      []byte
}

// NewWriter creates path and starts the background goroutine that drains lines into
// it. Call Close when the block tears down to flush and release the file.
func NewWriter(path string) (*Writer, error) {
	if path == "" {
		return nil, fmt.Errorf("logcapture: a writer needs a file path")
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("logcapture: creating %q: %w", path, err)
	}
	w := &Writer{
		lines: make(chan logLine, bufferedLines),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go w.run(f)
	return w, nil
}

func (w *Writer) run(f *os.File) {
	defer close(w.done)
	defer func() { _ = f.Close() }()

	for {
		// Checked before the blocking select on every iteration so a saturated lines
		// channel (a container logging continuously) can never starve stop out of ever
		// being noticed — Go's select has no priority between two simultaneously ready
		// cases, so without this check Close could be delayed indefinitely.
		select {
		case <-w.stop:
			w.finish(f)
			return
		default:
		}

		select {
		case l := <-w.lines:
			w.writeLine(f, l)
		case <-w.stop:
			w.finish(f)
			return
		}
	}
}

// finish drains whatever remains buffered and appends a dropped-line summary, run once
// stop has been observed.
func (w *Writer) finish(f *os.File) {
	w.drain(f)
	if dropped := w.dropped.Load(); dropped > 0 {
		_, _ = fmt.Fprintf(f, "[logcapture] dropped %d line(s): writer fell behind the containers' combined output\n", dropped)
	}
}

// drain flushes whatever is already buffered, without waiting for more — Accept keeps
// working after this (a late line is simply never read), so it can never block.
func (w *Writer) drain(f *os.File) {
	for {
		select {
		case l := <-w.lines:
			w.writeLine(f, l)
		default:
			return
		}
	}
}

func (w *Writer) writeLine(f *os.File, l logLine) {
	text := l.text
	if len(text) == 0 || text[len(text)-1] != '\n' {
		text = append(append([]byte(nil), text...), '\n')
	}
	_, _ = fmt.Fprintf(f, "[%s] ", l.component)
	_, _ = f.Write(text)
}

// Consumer returns a testcontainers.LogConsumer that forwards a single component's
// log lines into this writer, tagged with component in the combined file.
func (w *Writer) Consumer(component string) testcontainers.LogConsumer {
	return &consumer{writer: w, component: component}
}

// Close signals the writer goroutine to drain and flush what has already arrived, then
// waits for it to finish and closes the file. Safe to call once per Writer, and safe to
// call while a container's log producer is still delivering lines — Accept remains a
// harmless no-op send into an unread channel afterward, never a panic or a block.
func (w *Writer) Close() {
	close(w.stop)
	<-w.done
}

// Dropped reports how many lines were dropped for arriving faster than the writer
// goroutine could drain the buffer.
func (w *Writer) Dropped() int64 {
	if w == nil {
		return 0
	}
	return w.dropped.Load()
}

type consumer struct {
	writer    *Writer
	component string
}

// Accept implements testcontainers.LogConsumer. It never blocks and never panics, even
// after the writer has been closed: a full (or abandoned) buffer drops the line and
// counts it instead of applying backpressure to the container's own log-producer
// goroutine.
//
// l.Content must be copied here, not retained by reference: testcontainers' own
// docker.go reuses one buffer across the whole log stream (see moby's stdcopy.StdCopy),
// so a queued line would otherwise be overwritten by a later one before this Writer's
// goroutine gets around to draining it.
func (c *consumer) Accept(l testcontainers.Log) {
	text := append([]byte(nil), l.Content...)
	select {
	case c.writer.lines <- logLine{component: c.component, text: text}:
	default:
		c.writer.dropped.Add(1)
	}
}
