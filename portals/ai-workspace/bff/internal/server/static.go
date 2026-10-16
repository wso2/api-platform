/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the
 * License at http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// assetsPrefix is Vite's build output directory, as it appears in a cleaned
// request path: everything under it is content-hashed and immutable.
const assetsPrefix = "/assets"

// spaHandler serves the built SPA: real files when they exist, otherwise it
// falls back to index.html so client-side routing works (replaces nginx
// try_files $uri /index.html). index.html is served no-store; hashed assets are
// cacheable.
func spaHandler(staticDir string) http.Handler {
	indexPath := filepath.Join(staticDir, "index.html")
	fileServer := http.FileServer(http.Dir(staticDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean and resolve the requested path within staticDir.
		clean := filepath.Clean(r.URL.Path)
		full := filepath.Join(staticDir, clean)

		// Prevent path traversal outside staticDir. Assert containment against
		// the root with a trailing separator so a sibling dir whose name merely
		// shares the prefix (e.g. root "/var/www" vs "/var/www-secret") cannot
		// pass a bare HasPrefix check.
		root := filepath.Clean(staticDir)
		if full != root && !strings.HasPrefix(full, root+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}

		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			if clean == "/index.html" || strings.HasSuffix(clean, "/index.html") {
				w.Header().Set("Cache-Control", "no-store")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		// A missing file under assets/ is NEVER a client-side route: those paths
		// are content-hashed build output, so one that is absent means the
		// index.html the browser is running was built against a different
		// bundle than this process serves (a half-finished rollout, or replicas
		// on two images). Answering index.html there returns 200 text/html for
		// a `<script type="module">`, which the browser rejects on MIME grounds
		// with an error naming neither the file nor the cause. 404 says it
		// plainly, and keeps a cacheable asset URL from holding an HTML body.
		if clean == assetsPrefix || strings.HasPrefix(clean, assetsPrefix+"/") {
			http.NotFound(w, r)
			return
		}

		// Fallback to index.html for SPA routes. Deliberately NOT narrowed by
		// Accept: a health probe or a curl of a client-side route sends no
		// Accept at all, and 404ing those to catch a mistyped asset path would
		// trade a clear failure for an outage.
		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, indexPath)
	})
}
