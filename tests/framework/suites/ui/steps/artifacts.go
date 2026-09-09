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

package steps

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	playwright "github.com/mxschmitt/playwright-go"

	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

// artifactsDir is where failing scenarios drop their evidence, relative to the suite's
// working directory (tests/framework/suites/ui). Gitignored.
const artifactsDir = "artifacts"

var artifactNameSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

func prepareArtifactsDir() error {
	if err := os.MkdirAll(artifactsDir, 0o700); err != nil {
		return err
	}
	return os.Chmod(artifactsDir, 0o700)
}

func markSensitiveArtifacts(ctx context.Context) error {
	return tcontext.Set(ctx, keySensitiveArtifacts, true)
}

func sensitiveArtifacts(ctx context.Context) bool {
	value, ok := tcontext.Get(ctx, keySensitiveArtifacts)
	marked, isBool := value.(bool)
	return ok && isBool && marked
}

// artifactBaseName is the shared stem for one failure's evidence files.
func artifactBaseName(scenario string) string {
	name := artifactNameSanitizer.ReplaceAllString(strings.ToLower(scenario), "-")
	return strings.Trim(name, "-") + "-" + time.Now().Format("150405")
}

// saveFailureArtifacts captures what the failing scenario's page looked like — a full-page
// screenshot and the DOM — so a red run leaves more behind than an assertion message.
// Best-effort: the scenario's own error is the signal, artifact trouble must not mask it.
func (u *UI) saveFailureArtifacts(ctx context.Context, name string) {
	if sensitiveArtifacts(ctx) {
		return
	}
	page, err := u.page(ctx)
	if err != nil {
		return
	}
	if err := prepareArtifactsDir(); err != nil {
		return
	}
	if shot, err := page.Screenshot(playwright.PageScreenshotOptions{
		FullPage: playwright.Bool(true)}); err == nil {
		_ = os.WriteFile(filepath.Join(artifactsDir, name+".png"), shot, 0o600)
	}
	if content, err := page.Content(); err == nil {
		_ = os.WriteFile(filepath.Join(artifactsDir, name+".html"), []byte(content), 0o600)
	}
}

// stopTracing ends the scenario's trace recording: kept as a zip beside the other
// artifacts when the scenario failed, discarded otherwise. Best-effort, like the rest.
func (u *UI) stopTracing(ctx context.Context, name string, keep bool) {
	v, ok := tcontext.Get(ctx, keyBrowserContext)
	if !ok {
		return
	}
	bctx, ok := v.(playwright.BrowserContext)
	if !ok {
		return
	}
	if !keep {
		_ = bctx.Tracing().Stop()
		return
	}
	if sensitiveArtifacts(ctx) {
		_ = bctx.Tracing().Stop()
		return
	}
	if err := prepareArtifactsDir(); err != nil {
		return
	}
	tracePath := filepath.Join(artifactsDir, name+"-trace.zip")
	if err := bctx.Tracing().Stop(tracePath); err == nil {
		_ = os.Chmod(tracePath, 0o600)
	}
}
