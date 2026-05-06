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

package funcs

import (
	"log/slog"
	"text/template"

	"github.com/wso2/api-platform/common/redact"
)

// SecretResolver abstracts secret retrieval so the funcs package
// can be tested without depending on the concrete SecretService.
type SecretResolver interface {
	// Resolve returns the plaintext value for the given secret handle.
	Resolve(handle string) (string, error)
}

// Deps carries shared dependencies that template functions may need.
type Deps struct {
	SecretResolver SecretResolver
	Tracker        *redact.SecretTracker
	Logger         *slog.Logger
}

// Func describes a single template function to register.
type Func struct {
	Name string
	Fn   func(deps *Deps) any
}

// registry collects all registered template functions.
var registry []Func

// Register adds a template function to the registry.
// Called from init() in each function file.
func Register(f Func) {
	registry = append(registry, f)
}

// BuildFuncMap creates a template.FuncMap from all registered functions.
func BuildFuncMap(deps *Deps) template.FuncMap {
	fm := make(template.FuncMap, len(registry))
	for _, f := range registry {
		fm[f.Name] = f.Fn(deps)
	}
	return fm
}
