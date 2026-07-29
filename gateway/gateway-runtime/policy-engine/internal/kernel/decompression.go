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
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
)

// ErrDecompressedTooLarge is returned when decompressed output exceeds the
// configured ceiling — the signature of a decompression bomb.
var ErrDecompressedTooLarge = errors.New("decompressed body exceeds maximum allowed size")

// decompressBody decompresses body bytes based on the Content-Encoding value.
// Supported encodings: "gzip", "br" (Brotli). Unknown encodings are returned as-is.
// Output is capped at maxBytes (<= 0 means unbounded); exceeding it returns
// ErrDecompressedTooLarge, never a truncated body.
func decompressBody(body []byte, encoding string, maxBytes int64) ([]byte, error) {
	switch encoding {
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		return readLimited(r, maxBytes)
	case "br":
		r := brotli.NewReader(bytes.NewReader(body))
		return readLimited(r, maxBytes)
	default:
		return body, nil
	}
}

// readLimited reads all of r, returning ErrDecompressedTooLarge if the input
// exceeds maxBytes — never a truncated result. maxBytes <= 0 means unbounded.
func readLimited(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(r)
	}
	// Read at most maxBytes first, then probe the underlying reader for one more
	// byte. This avoids maxBytes+1 overflowing when maxBytes is math.MaxInt64.
	data, err := io.ReadAll(io.LimitReader(r, maxBytes))
	if err != nil {
		return nil, err
	}
	var extra [1]byte
	n, err := r.Read(extra[:])
	if n > 0 {
		return nil, ErrDecompressedTooLarge
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return data, nil
}

// decoderInputChunk is one compressed input chunk and a boundary notification.
// consumed is closed when the decoder asks for input beyond this chunk, after all
// output it could produce without more input has been queued.
type decoderInputChunk struct {
	data     []byte
	consumed chan struct{}
}

// decoderInput is a resumable io.Reader for the persistent gzip/brotli decoder.
// Unlike io.Pipe, it exposes deterministic chunk boundaries: the decoder cannot
// receive the next chunk until FeedChunk has drained all output from the previous
// one.
type decoderInput struct {
	chunks chan decoderInputChunk
	done   chan struct{}
	once   sync.Once
	err    error

	// current and consumed are owned exclusively by the decoder goroutine.
	current  []byte
	consumed chan struct{}
}

func newDecoderInput() *decoderInput {
	return &decoderInput{
		chunks: make(chan decoderInputChunk),
		done:   make(chan struct{}),
	}
}

func (in *decoderInput) Read(p []byte) (int, error) {
	// An error close must abort even when part of the current compressed chunk
	// remains, otherwise a bomb would continue consuming CPU after detection.
	select {
	case <-in.done:
		return 0, in.closeError()
	default:
	}

	if len(in.current) == 0 {
		if in.consumed != nil {
			close(in.consumed)
			in.consumed = nil
		}

		select {
		case <-in.done:
			return 0, in.closeError()
		case chunk := <-in.chunks:
			in.current = chunk.data
			in.consumed = chunk.consumed
		}
	}

	n := copy(p, in.current)
	in.current = in.current[n:]
	return n, nil
}

func (in *decoderInput) closeWithError(err error) {
	in.once.Do(func() {
		in.err = err
		close(in.done)
	})
}

func (in *decoderInput) closeError() error {
	if in.err != nil {
		return in.err
	}
	return io.EOF
}

// streamDecompressor provides true per-chunk streaming decompression using a
// persistent decoder goroutine. Incoming compressed chunks are handed to a
// boundary-aware reader; the goroutine owns the stateful gzip/brotli reader and
// pushes decompressed bytes to an output channel as complete blocks become
// available.
type streamDecompressor struct {
	input       *decoderInput
	outChan     chan []byte
	errChan     chan error
	decoderDone chan struct{}

	maxBytes int64
}

// newStreamDecompressor starts the background decoder goroutine and returns a
// streamDecompressor ready to accept chunks via FeedChunk. maxBytes caps the
// decompressed output of each FeedChunk call (<= 0 disables the cap).
func newStreamDecompressor(encoding string, maxBytes int64) *streamDecompressor {
	input := newDecoderInput()
	// Buffer a bounded number of decoder blocks so the decoder can continue working
	// while FeedChunk handles its boundary notification. At
	// the default 32 KiB block size this queue is capped at 2 MiB; for ceilings below
	// 32 KiB the block size is reduced below, shrinking the queue proportionally.
	outChan := make(chan []byte, 64)
	errChan := make(chan error, 1)
	decoderDone := make(chan struct{})

	go func() {
		defer close(outChan)
		defer close(decoderDone)
		var r io.Reader
		switch encoding {
		case "gzip":
			gr, err := gzip.NewReader(input)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("gzip.NewReader: %w", err):
				default:
				}
				return
			}
			defer gr.Close()
			r = gr
		case "br":
			r = brotli.NewReader(input)
		default:
			r = input
		}

		bufSize := 32 * 1024
		if maxBytes > 0 && maxBytes < int64(bufSize) {
			// Reading at most maxBytes+1 lets appendOutput detect overflow while
			// keeping the decoder's single in-flight block proportional to a small
			// configured ceiling.
			bufSize = int(maxBytes) + 1
		}
		buf := make([]byte, bufSize)
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

	return &streamDecompressor{
		input:       input,
		outChan:     outChan,
		errChan:     errChan,
		decoderDone: decoderDone,
		maxBytes:    maxBytes,
	}
}

// FeedChunk writes compressed bytes into the decoder and returns whatever decompressed
// bytes are immediately available.
//
// For intermediate chunks the output may be empty — the decoder needs more input before
// a full DEFLATE/brotli block can be decoded. Callers must tolerate empty output.
//
// On endOfStream=true the decoder input is closed and FeedChunk blocks until all
// remaining decompressed bytes have been flushed by the goroutine.
//
// Decompressed output per call is capped at sd.maxBytes; crossing the cap returns
// ErrDecompressedTooLarge. The cap is per call, not cumulative, so long-lived
// streams keep flowing while peak memory stays bounded. After an error the caller
// must Close() the decompressor to drain the goroutine.
func (sd *streamDecompressor) FeedChunk(chunk []byte, endOfStream bool) ([]byte, error) {
	var result []byte

	if len(chunk) > 0 {
		consumed := make(chan struct{})
		inputChunk := decoderInputChunk{data: chunk, consumed: consumed}
		select {
		case sd.input.chunks <- inputChunk:
		case <-sd.input.done:
			return nil, fmt.Errorf("stream decompressor input: %w", sd.input.closeError())
		case <-sd.decoderDone:
			return nil, sd.decoderError()
		}

		// Wait until the decoder requests input beyond this chunk: once consumed
		// closes, every output block the decoder could produce from the available
		// input is already queued.
		for consumed != nil {
			select {
			case data, ok := <-sd.outChan:
				if !ok {
					return result, sd.decoderError()
				}
				if err := sd.appendOutput(&result, data); err != nil {
					return nil, err
				}
			case <-consumed:
				consumed = nil
			}
		}
	}

	if endOfStream {
		sd.input.closeWithError(nil)
		for data := range sd.outChan {
			if err := sd.appendOutput(&result, data); err != nil {
				return nil, err
			}
		}
		if err := sd.decoderError(); err != nil {
			return result, err
		}
		return result, nil
	}

	// The consumed boundary guarantees no more output can be queued until the next
	// input chunk arrives, so this non-blocking drain sees everything available.
	for {
		select {
		case data, ok := <-sd.outChan:
			if !ok {
				// Decoder exited before end-of-stream — surface its error so the
				// caller can reject the stream.
				if err := sd.decoderError(); err != nil {
					return result, err
				}
				return result, nil
			}
			if err := sd.appendOutput(&result, data); err != nil {
				return nil, err
			}
		default:
			return result, nil
		}
	}
}

// appendOutput appends one decoder block to result, enforcing the per-call
// maxBytes cap. Closing the input on overflow aborts the decoder promptly.
func (sd *streamDecompressor) appendOutput(result *[]byte, data []byte) error {
	if sd.maxBytes > 0 && int64(len(data)) > sd.maxBytes-int64(len(*result)) {
		sd.input.closeWithError(ErrDecompressedTooLarge)
		return ErrDecompressedTooLarge
	}
	*result = append(*result, data...)
	return nil
}

func (sd *streamDecompressor) decoderError() error {
	select {
	case err := <-sd.errChan:
		return err
	default:
		return nil
	}
}

// Close releases the decompressor's input and drains the goroutine. Call this on
// error paths where endOfStream will never arrive.
func (sd *streamDecompressor) Close() {
	sd.input.closeWithError(io.ErrClosedPipe)
	for range sd.outChan {
	}
}

// recompressBody re-compresses body bytes using the original Content-Encoding.
// Used to restore compression after policies have processed the decompressed body.
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
