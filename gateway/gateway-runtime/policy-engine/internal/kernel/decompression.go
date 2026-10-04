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
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
)

// Content-coding tokens the kernel can round-trip. These are the values stored
// in requestContentEncoding/responseContentEncoding — already lowercased, and
// for "deflate" already resolved to one of the two wire variants below.
const (
	encodingGzip = "gzip"
	encodingBr   = "br"
	encodingZstd = "zstd"
	// encodingDeflate is "deflate" in its RFC 9110 / RFC 1950 form: DEFLATE data
	// inside a zlib wrapper.
	encodingDeflate = "deflate"
	// encodingDeflateRaw is an INTERNAL token, never a wire value. Some servers
	// and clients send "Content-Encoding: deflate" carrying bare RFC 1951 DEFLATE
	// with no zlib wrapper. Both are decodable, but the two are not interchangeable
	// on output — re-encoding raw input as zlib-wrapped (or vice versa) hands the
	// peer a body its decoder rejects. Recording which variant arrived lets the
	// kernel emit the same one back; the Content-Encoding header itself is never
	// rewritten and stays "deflate" either way.
	encodingDeflateRaw = "deflate-raw"
	// encodingIdentity is the no-op coding: present in the header but meaning the
	// body is not encoded at all.
	encodingIdentity = "identity"
)

// zstdDecoderConcurrency/zstdEncoderConcurrency pin the zstd codec to a single
// goroutine per stream. The library defaults to GOMAXPROCS workers per
// encoder/decoder, which on a proxy handling many concurrent bodies multiplies
// into thousands of goroutines for no throughput gain at these body sizes.
const (
	zstdDecoderConcurrency = 1
	zstdEncoderConcurrency = 1
)

// deflateVariantProbeBytes is how many leading compressed bytes are needed to
// tell zlib-wrapped deflate from raw deflate: the RFC 1950 header is two bytes.
const deflateVariantProbeBytes = 2

// needsMoreDeflateVariantEvidence reports whether a streaming decoder for this
// encoding must wait for more compressed input before it can be constructed.
// Only "deflate" is ambiguous, and only until deflateVariantProbeBytes have
// accumulated; at end-of-stream nothing more is coming, so whatever arrived is
// what the decision is made on.
func needsMoreDeflateVariantEvidence(encoding string, buffered []byte, endOfStream bool) bool {
	return encoding == encodingDeflate && !endOfStream && len(buffered) < deflateVariantProbeBytes
}

// resolveDeflateVariant inspects the first bytes of a "deflate" body and reports
// the concrete variant token to record for it.
//
// A zlib stream (RFC 1950) starts with a 2-byte header: the low nibble of the
// first byte is the compression method (8 == DEFLATE) and the big-endian pair is
// a multiple of 31. Bare DEFLATE data effectively never satisfies both, so this
// check distinguishes the two reliably. Too few bytes to tell yet falls back to
// the RFC-conformant zlib form — a streaming caller must instead buffer
// deflateVariantProbeBytes before deciding (see needsMoreDeflateVariantEvidence),
// because a first chunk of one byte would otherwise pin a raw-deflate stream to
// the zlib decoder for good.
func resolveDeflateVariant(body []byte) string {
	if len(body) < deflateVariantProbeBytes {
		return encodingDeflate
	}
	if body[0]&0x0f == 0x08 && (uint16(body[0])<<8|uint16(body[1]))%31 == 0 {
		return encodingDeflate
	}
	return encodingDeflateRaw
}

// zstdDecoderOptions builds the zstd decoder options for a body ceiling of
// maxBytes. WithDecoderMaxMemory doubles as the window-size ceiling for
// streaming decodes, so a frame *declaring* a window larger than the ceiling is
// rejected before the decoder allocates a buffer for it — readLimited alone
// only catches a bomb after that allocation has already happened. maxBytes <= 0
// means unbounded, leaving the library default in place. A ceiling below
// zstd.MinWindowSize is raised to it: every conformant frame declares at least a
// 1 KiB window, so a lower cap would reject all zstd bodies rather than just
// hostile ones.
func zstdDecoderOptions(maxBytes int64) []zstd.DOption {
	opts := []zstd.DOption{zstd.WithDecoderConcurrency(zstdDecoderConcurrency)}
	if maxBytes > 0 {
		maxMemory := uint64(maxBytes)
		if maxMemory < zstd.MinWindowSize {
			maxMemory = zstd.MinWindowSize
		}
		opts = append(opts, zstd.WithDecoderMaxMemory(maxMemory))
	}
	return opts
}

// ErrDecompressedTooLarge is returned when decompressed output exceeds the
// configured ceiling — the signature of a decompression bomb.
var ErrDecompressedTooLarge = errors.New("decompressed body exceeds maximum allowed size")

// decompressBody decompresses body bytes based on the Content-Encoding value.
// Supported encodings: gzip, br, zstd, and both deflate variants. Unknown
// encodings are returned as-is — callers must not reach this with one, since
// isRecompressibleEncoding gates every call site (an unsupported encoding is
// rejected outright rather than handed to policies as opaque bytes).
// Output is capped at maxBytes (<= 0 means unbounded); exceeding it returns
// ErrDecompressedTooLarge, never a truncated body.
func decompressBody(body []byte, encoding string, maxBytes int64) ([]byte, error) {
	switch encoding {
	case encodingGzip:
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer r.Close()
		return readLimited(r, maxBytes)
	case encodingBr:
		r := brotli.NewReader(bytes.NewReader(body))
		return readLimited(r, maxBytes)
	case encodingZstd:
		r, err := zstd.NewReader(bytes.NewReader(body), zstdDecoderOptions(maxBytes)...)
		if err != nil {
			return nil, fmt.Errorf("zstd reader: %w", err)
		}
		defer r.Close()
		return readLimited(r, maxBytes)
	case encodingDeflate:
		r, err := zlib.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("deflate (zlib) reader: %w", err)
		}
		defer r.Close()
		return readLimited(r, maxBytes)
	case encodingDeflateRaw:
		r := flate.NewReader(bytes.NewReader(body))
		defer r.Close()
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
		// gzip/zlib/zstd readers consume their stream header eagerly, so
		// construction blocks here until the first chunk arrives. That is why the
		// decoder lives on its own goroutine: newStreamDecompressor must return
		// before any body byte has been seen.
		var r io.Reader
		switch encoding {
		case encodingGzip:
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
		case encodingBr:
			r = brotli.NewReader(input)
		case encodingZstd:
			zr, err := zstd.NewReader(input, zstdDecoderOptions(maxBytes)...)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("zstd.NewReader: %w", err):
				default:
				}
				return
			}
			defer zr.Close()
			r = zr
		case encodingDeflate:
			zr, err := zlib.NewReader(input)
			if err != nil {
				select {
				case errChan <- fmt.Errorf("zlib.NewReader: %w", err):
				default:
				}
				return
			}
			defer zr.Close()
			r = zr
		case encodingDeflateRaw:
			fr := flate.NewReader(input)
			defer fr.Close()
			r = fr
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

// ─── Streaming re-compression ────────────────────────────────────────────────
//
// A streamed response must be re-compressed as ONE compressed stream spanning
// the whole body, not one per chunk. Calling recompressBody per chunk produces
// N independent members: for gzip that is a multi-member stream, and a client
// that stops at the end of the first member silently sees a truncated body
// (multi-member support is not something a server may assume — Go's gzip reader
// happens to be multistream by default, others are not); for brotli, which has
// no multi-member concatenation at all, the remainder is undecodable.
//
// streamCompressor keeps a single writer alive for the lifetime of the response
// and flushes after each chunk so data still reaches the client incrementally.
type streamCompressor struct {
	encoding string
	buf      bytes.Buffer
	// w and flush are the encoder for this stream. Every supported codec exposes
	// Write/Close/Flush, so they are held behind these two fields rather than one
	// typed field per codec — a per-codec field forces every method here to grow a
	// new case, and a missed one silently degrades to "no compression applied".
	w      io.WriteCloser
	flush  func() error
	closed bool
}

// newStreamCompressor returns a compressor for the encoding, or nil when the
// encoding needs no re-compression (callers then forward bytes unchanged).
// Encodings must be pre-validated with isRecompressibleEncoding; nil here means
// "forward untouched", which is only correct for an unencoded body.
func newStreamCompressor(encoding string) *streamCompressor {
	sc := &streamCompressor{encoding: encoding}
	switch encoding {
	case encodingGzip:
		w := gzip.NewWriter(&sc.buf)
		sc.w, sc.flush = w, w.Flush
	case encodingBr:
		w := brotli.NewWriter(&sc.buf)
		sc.w, sc.flush = w, w.Flush
	case encodingZstd:
		// Error is unreachable: it reports invalid encoder options, and the
		// options here are compile-time constants.
		w, err := zstd.NewWriter(&sc.buf, zstd.WithEncoderConcurrency(zstdEncoderConcurrency))
		if err != nil {
			return nil
		}
		sc.w, sc.flush = w, w.Flush
	case encodingDeflate:
		w := zlib.NewWriter(&sc.buf)
		sc.w, sc.flush = w, w.Flush
	case encodingDeflateRaw:
		// Error is unreachable: it reports an out-of-range level, and the level
		// here is a library constant.
		w, err := flate.NewWriter(&sc.buf, flate.DefaultCompression)
		if err != nil {
			return nil
		}
		sc.w, sc.flush = w, w.Flush
	default:
		return nil
	}
	return sc
}

// Compress writes one chunk into the single ongoing compressed stream and
// returns the bytes produced so far. When endOfStream is set the stream is
// finalised (footer/checksum written) and the compressor must not be reused.
//
// A flush is emitted per chunk so the client receives data incrementally; this
// costs a few bytes of framing per chunk versus a single whole-body compress,
// which is the correct trade for a streaming response.
func (sc *streamCompressor) Compress(body []byte, endOfStream bool) ([]byte, error) {
	if sc.closed {
		return nil, fmt.Errorf("%s stream compressor already closed", sc.encoding)
	}
	sc.buf.Reset()

	if len(body) > 0 {
		if _, err := sc.w.Write(body); err != nil {
			return nil, fmt.Errorf("%s write: %w", sc.encoding, err)
		}
	}
	if endOfStream {
		if err := sc.w.Close(); err != nil {
			return nil, fmt.Errorf("%s close: %w", sc.encoding, err)
		}
		sc.closed = true
	} else if err := sc.flush(); err != nil {
		return nil, fmt.Errorf("%s flush: %w", sc.encoding, err)
	}

	out := make([]byte, sc.buf.Len())
	copy(out, sc.buf.Bytes())
	return out, nil
}

// Close finalises the stream on error paths where endOfStream never arrives.
func (sc *streamCompressor) Close() {
	if sc.closed {
		return
	}
	sc.closed = true
	_ = sc.w.Close()
}

// isRecompressibleEncoding reports whether the kernel can decompress and
// re-compress this Content-Encoding. Anything else is rejected outright by
// execution_context.go: the kernel neither runs body policies on bytes they
// cannot read nor forwards a body it could not have inspected.
func isRecompressibleEncoding(encoding string) bool {
	switch encoding {
	case encodingGzip, encodingBr, encodingZstd, encodingDeflate, encodingDeflateRaw:
		return true
	default:
		return false
	}
}

// recompressBody re-compresses body bytes using the original Content-Encoding.
// Used for the BUFFERED response path, where the whole body is compressed in a
// single call. Streaming responses must use streamCompressor instead so the
// response is one compressed stream rather than one per chunk.
// Supported encodings: gzip, br, zstd, and both deflate variants. Unknown
// encodings are returned as-is; call sites are gated by isRecompressibleEncoding
// so that case is unreachable for a body policies actually touched.
func recompressBody(body []byte, encoding string) ([]byte, error) {
	// Reuse the streaming encoder in a single write+finalise, so the buffered and
	// streaming paths cannot drift apart on which encodings they support or how
	// each one is framed.
	sc := newStreamCompressor(encoding)
	if sc == nil {
		// An encoding this function does not encode at all: return the body
		// untouched, as it always has.
		if !isRecompressibleEncoding(encoding) {
			return body, nil
		}
		// A supported encoding whose codec refused its options. Returning the body
		// here would emit plaintext under a compressed Content-Encoding header —
		// the exact corruption this file exists to prevent.
		return nil, fmt.Errorf("no compressor available for encoding %q", encoding)
	}
	return sc.Compress(body, true)
}
