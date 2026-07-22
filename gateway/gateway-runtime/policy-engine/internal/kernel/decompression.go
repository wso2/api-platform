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

package kernel

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
)

// decompressBody decompresses body bytes based on the Content-Encoding value.
// Supported encodings: "gzip", "br" (Brotli). Unknown encodings are returned as-is.
func decompressBody(body []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		return io.ReadAll(r)
	case "br":
		r := brotli.NewReader(bytes.NewReader(body))
		return io.ReadAll(r)
	default:
		return body, nil
	}
}

// streamDecompressor provides true per-chunk streaming decompression. It adapts
// Envoy's push-style chunk delivery to Go's pull-style gzip/brotli reader: a single
// decoder goroutine owns a stateful reader that pulls compressed bytes from an
// internal buffer (fed by FeedChunk) and writes decompressed bytes to an unbounded
// output buffer.
//
// The synchronization is a single sync.Cond guarding shared state. Two invariants make
// it deadlock-free and race-free, unlike the previous io.Pipe + bounded-channel design:
//
//  1. The decoder goroutine NEVER blocks on output — decompressed bytes go into an
//     unbounded bytes.Buffer that FeedChunk drains on every call. (The old design used
//     a bounded channel; when it filled, the goroutine blocked mid-decode, stopped
//     consuming input, and the whole stream wedged. That was the mid-stream stall.)
//  2. FeedChunk returns exactly when the decoder has consumed all input fed so far and
//     is blocked waiting for more (feederBlocked), or has finished/errored (done). At
//     that point every byte decodable from the fed input is already in the output
//     buffer — so output is complete per chunk, with no racy "maybe next time" drain.
type streamDecompressor struct {
	mu   sync.Mutex
	cond *sync.Cond

	inbuf         []byte       // compressed bytes fed but not yet consumed by the decoder
	out           bytes.Buffer // decompressed bytes not yet returned to the caller (unbounded)
	closed        bool         // endOfStream fed: feeder returns io.EOF once inbuf drains
	feederBlocked bool         // decoder is parked waiting for more input (inbuf empty)
	done          bool         // decoder goroutine has exited
	decodeErr     error        // terminal decode error (nil on clean io.EOF)
	passthrough   bool         // unknown encoding: FeedChunk returns bytes unchanged
}

// newStreamDecompressor starts the background decoder goroutine and returns a
// streamDecompressor ready to accept chunks via FeedChunk.
func newStreamDecompressor(encoding string) *streamDecompressor {
	sd := &streamDecompressor{}
	sd.cond = sync.NewCond(&sd.mu)

	if encoding != "gzip" && encoding != "br" {
		sd.passthrough = true
		return sd
	}

	go sd.decodeLoop(encoding)
	return sd
}

// feederReader is the io.Reader handed to the gzip/brotli decoder. Each Read hands the
// decoder whatever compressed bytes are currently buffered; when the buffer is empty it
// parks (recording feederBlocked=true so FeedChunk knows all fed input is consumed) until
// FeedChunk supplies more or signals end-of-stream.
type feederReader struct{ sd *streamDecompressor }

func (f feederReader) Read(p []byte) (int, error) {
	sd := f.sd
	sd.mu.Lock()
	defer sd.mu.Unlock()
	for len(sd.inbuf) == 0 {
		if sd.closed {
			return 0, io.EOF
		}
		sd.feederBlocked = true
		sd.cond.Broadcast() // wake any FeedChunk waiting for "all input consumed"
		sd.cond.Wait()
	}
	sd.feederBlocked = false
	n := copy(p, sd.inbuf)
	sd.inbuf = sd.inbuf[n:]
	return n, nil
}

// decodeLoop owns the stateful decoder for the life of the stream and copies its output
// into the unbounded buffer. It exits on io.EOF (clean) or any decode error.
func (sd *streamDecompressor) decodeLoop(encoding string) {
	var r io.Reader
	switch encoding {
	case "gzip":
		gr, err := gzip.NewReader(feederReader{sd: sd})
		if err != nil {
			sd.finish(fmt.Errorf("gzip.NewReader: %w", err))
			return
		}
		defer gr.Close()
		r = gr
	case "br":
		r = brotli.NewReader(feederReader{sd: sd})
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			sd.mu.Lock()
			sd.out.Write(buf[:n]) // bytes.Buffer copies, so reusing buf is safe
			sd.mu.Unlock()
		}
		if err == io.EOF {
			sd.finish(nil)
			return
		}
		if err != nil {
			sd.finish(err)
			return
		}
	}
}

// finish records the decoder's terminal state and wakes any waiting FeedChunk.
func (sd *streamDecompressor) finish(err error) {
	sd.mu.Lock()
	sd.decodeErr = err
	sd.done = true
	sd.cond.Broadcast()
	sd.mu.Unlock()
}

// FeedChunk feeds compressed bytes to the decoder and returns every decompressed byte
// derivable from the input fed so far. Output may legitimately be empty for an
// intermediate chunk when the decoder needs more input to complete a block.
//
// It blocks only until the decoder has consumed all fed input and parked for more (or
// finished) — bounded by the CPU cost of decoding this chunk, never on downstream
// delivery — so it cannot wedge the ext_proc stream.
func (sd *streamDecompressor) FeedChunk(chunk []byte, endOfStream bool) ([]byte, error) {
	if sd.passthrough {
		return chunk, nil
	}

	sd.mu.Lock()
	defer sd.mu.Unlock()

	if len(chunk) > 0 {
		sd.inbuf = append(sd.inbuf, chunk...)
	}
	if endOfStream {
		sd.closed = true
	}
	sd.feederBlocked = false
	sd.cond.Broadcast() // wake the feeder to consume the new input / observe close

	// Wait until the decoder has drained all fed input and parked (feederBlocked with an
	// empty inbuf), or has exited. Either way, all output decodable from the fed input is
	// now in sd.out.
	for !sd.done && !(sd.feederBlocked && len(sd.inbuf) == 0) {
		sd.cond.Wait()
	}

	result := make([]byte, sd.out.Len())
	copy(result, sd.out.Bytes())
	sd.out.Reset()

	if sd.decodeErr != nil && sd.decodeErr != io.EOF {
		return result, sd.decodeErr
	}
	return result, nil
}

// Close releases the decoder goroutine on error paths where endOfStream will never
// arrive. It signals end-of-input and returns without joining, so it never hangs; the
// goroutine observes the close, drains, and exits on its own. Safe to call multiple times.
func (sd *streamDecompressor) Close() {
	if sd.passthrough {
		return
	}
	sd.mu.Lock()
	sd.closed = true
	sd.cond.Broadcast()
	sd.mu.Unlock()
}

// streamWriteFlushCloser is the common interface implemented by *gzip.Writer and
// *brotli.Writer: incremental writes, a mid-stream flush that keeps the stream open,
// and a final close that emits the trailer.
type streamWriteFlushCloser interface {
	io.WriteCloser
	Flush() error
}

// streamCompressor provides stateful, per-chunk streaming compression that produces
// a SINGLE continuous compressed stream across all chunks.
//
// This is the compression counterpart of streamDecompressor and must be used instead
// of recompressBody for streaming bodies. recompressBody creates a fresh writer and
// Close()es it on every call, so each chunk becomes an independent, self-contained
// gzip/brotli member. A concatenation of independent members is not what the
// Content-Encoding header promises (one stream for the whole body), and downstream
// HTTP decoders (e.g. the Anthropic/Claude Code client) decode only the first member,
// see its end-of-stream trailer, and treat the response as finished — dropping every
// subsequent chunk. streamCompressor instead keeps one writer alive for the life of
// the stream, flushing (Z_SYNC_FLUSH) after each chunk and closing only at EndOfStream.
type streamCompressor struct {
	buf      *bytes.Buffer
	writer   streamWriteFlushCloser
	encoding string
	closed   bool
}

// newStreamCompressor returns a streamCompressor for the given Content-Encoding.
// For unknown encodings the writer is nil and FeedChunk passes bytes through unchanged.
func newStreamCompressor(encoding string) *streamCompressor {
	buf := &bytes.Buffer{}
	switch encoding {
	case "gzip":
		return &streamCompressor{buf: buf, writer: gzip.NewWriter(buf), encoding: encoding}
	case "br":
		return &streamCompressor{buf: buf, writer: brotli.NewWriter(buf), encoding: encoding}
	default:
		return &streamCompressor{buf: buf, encoding: encoding}
	}
}

// FeedChunk compresses a chunk of the decompressed body and returns the compressed
// bytes produced so far. On intermediate chunks the writer is flushed (keeping the
// stream open); on endOfStream the writer is closed, emitting the final block and
// trailer. The returned bytes belong to the caller (a fresh copy).
func (sc *streamCompressor) FeedChunk(chunk []byte, endOfStream bool) ([]byte, error) {
	// Passthrough for unknown encodings.
	if sc.writer == nil {
		return chunk, nil
	}
	if sc.closed {
		return nil, fmt.Errorf("stream compressor: write after close")
	}
	if len(chunk) > 0 {
		if _, err := sc.writer.Write(chunk); err != nil {
			return nil, fmt.Errorf("stream compressor write: %w", err)
		}
	}
	if endOfStream {
		if err := sc.writer.Close(); err != nil {
			return nil, fmt.Errorf("stream compressor close: %w", err)
		}
		sc.closed = true
	} else {
		if err := sc.writer.Flush(); err != nil {
			return nil, fmt.Errorf("stream compressor flush: %w", err)
		}
	}
	out := make([]byte, sc.buf.Len())
	copy(out, sc.buf.Bytes())
	sc.buf.Reset()
	return out, nil
}

// Close releases the compressor's writer on error paths where endOfStream will never
// arrive. Safe to call multiple times.
func (sc *streamCompressor) Close() {
	if sc.writer != nil && !sc.closed {
		_ = sc.writer.Close()
		sc.closed = true
	}
}

// recompressBody re-compresses body bytes using the original Content-Encoding.
// Used to restore compression after policies have processed the decompressed body.
// NOTE: This produces a complete standalone stream and is only correct for
// non-streaming (fully buffered) bodies. For streaming bodies use streamCompressor,
// which keeps a single stream open across chunks.
// Supported encodings: "gzip", "br" (Brotli). Unknown encodings are returned as-is.
func recompressBody(body []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "gzip":
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		if _, err := w.Write(body); err != nil {
			return nil, fmt.Errorf("gzip write: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("gzip close: %w", err)
		}
		return buf.Bytes(), nil
	case "br":
		var buf bytes.Buffer
		w := brotli.NewWriter(&buf)
		if _, err := w.Write(body); err != nil {
			return nil, fmt.Errorf("brotli write: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("brotli close: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return body, nil
	}
}
