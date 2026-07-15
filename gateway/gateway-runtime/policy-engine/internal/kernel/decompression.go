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
	"runtime"

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

// streamDecompressor provides true per-chunk streaming decompression using an io.Pipe
// and a persistent decoder goroutine. Incoming compressed chunks are written to the
// pipe; the goroutine owns the stateful gzip/brotli reader and pushes decompressed
// bytes to an output channel as complete blocks become available.
type streamDecompressor struct {
	pipeWriter *io.PipeWriter
	outChan    chan []byte
	errChan    chan error
}

// newStreamDecompressor starts the background decoder goroutine and returns a
// streamDecompressor ready to accept chunks via FeedChunk.
func newStreamDecompressor(encoding string) *streamDecompressor {
	pr, pw := io.Pipe()
	outChan := make(chan []byte, 64)
	errChan := make(chan error, 1)

	go func() {
		defer close(outChan)
		var r io.Reader
		switch encoding {
		case "gzip":
			gr, err := gzip.NewReader(pr)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("gzip.NewReader: %w", err):
				default:
				}
				_ = pr.CloseWithError(err)
				return
			}
			defer gr.Close()
			r = gr
		case "br":
			r = brotli.NewReader(pr)
		default:
			r = pr
		}

		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				out := make([]byte, n)
				copy(out, buf[:n])
				outChan <- out
			}
			if err == io.EOF {
				return
			}
			if err != nil {
				select {
				case errChan <- err:
				default:
				}
				return
			}
		}
	}()

	return &streamDecompressor{pipeWriter: pw, outChan: outChan, errChan: errChan}
}

// FeedChunk writes compressed bytes into the decoder and returns whatever decompressed
// bytes are immediately available.
//
// For intermediate chunks the output may be empty — the decoder needs more input before
// a full DEFLATE/brotli block can be decoded. Callers must tolerate empty output.
//
// On endOfStream=true the pipe writer is closed and FeedChunk blocks until all remaining
// decompressed bytes have been flushed by the goroutine.
func (sd *streamDecompressor) FeedChunk(chunk []byte, endOfStream bool) ([]byte, error) {
	if len(chunk) > 0 {
		if _, err := sd.pipeWriter.Write(chunk); err != nil {
			return nil, fmt.Errorf("stream decompressor write: %w", err)
		}
	}

	if endOfStream {
		_ = sd.pipeWriter.Close()
		var result []byte
		for data := range sd.outChan {
			result = append(result, data...)
		}
		select {
		case err := <-sd.errChan:
			if err != nil {
				return result, err
			}
		default:
		}
		return result, nil
	}

	// pw.Write blocks until the goroutine's pr.Read has consumed all our bytes.
	// Yield so the goroutine can finish its r.Read call and push decoded output to
	// outChan before we do the non-blocking drain below.
	runtime.Gosched()

	var result []byte
	for {
		select {
		case data, ok := <-sd.outChan:
			if !ok {
				return result, nil
			}
			result = append(result, data...)
		default:
			return result, nil
		}
	}
}

// Close releases the decompressor's pipe and drains the goroutine. Call this on
// error paths where endOfStream will never arrive.
func (sd *streamDecompressor) Close() {
	_ = sd.pipeWriter.CloseWithError(io.ErrClosedPipe)
	for range sd.outChan {
	}
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
