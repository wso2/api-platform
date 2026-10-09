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

package translate

import "fmt"

// Warning records one lossy decision a Step took: a field the target gateway
// does not know that was stripped, or a value that could not be adapted and
// was shipped unchanged for the gateway to judge. Msg never carries
// credentials or upstream URLs; it names fields and versions only.
type Warning struct {
	Kind  string // artifact kind, e.g. "Mcp"
	Field string // YAML path of the affected field, e.g. "spec.upstream.url"
	Msg   string // one sentence
}

// String renders the warning for a log line or an error message.
func (w Warning) String() string {
	return fmt.Sprintf("%s %s: %s", w.Kind, w.Field, w.Msg)
}

// Report collects the warnings one translation produced. The translator stays
// free of logging: the deploy service that owns the deployment id and the
// gateway logs the report (see service.LogTranslationWarnings).
type Report struct {
	warnings []Warning
}

// Warn records a warning. A nil receiver is a no-op so a Step can be called
// without a report in tests.
func (r *Report) Warn(kind, field, format string, args ...any) {
	if r == nil {
		return
	}
	r.warnings = append(r.warnings, Warning{Kind: kind, Field: field, Msg: fmt.Sprintf(format, args...)})
}

// Warnings returns a copy of the recorded warnings, in the order they were
// recorded. It is nil when nothing lossy happened.
func (r *Report) Warnings() []Warning {
	if r == nil || len(r.warnings) == 0 {
		return nil
	}
	out := make([]Warning, len(r.warnings))
	copy(out, r.warnings)
	return out
}

// Empty reports whether the translation was lossless.
func (r *Report) Empty() bool {
	return r == nil || len(r.warnings) == 0
}
