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
