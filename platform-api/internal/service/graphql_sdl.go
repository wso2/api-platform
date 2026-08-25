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

package service

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// validateGraphQLSDL parses and validates a directly-supplied GraphQL SDL
// document. It rejects malformed SDL and SDL with no Query type. The
// returned error is for internal logging only — callers must map it to the
// generic GraphQLAPISchemaResolveFailed client response rather than
// surfacing the raw parser message (error-handling.md directive 1: a
// GraphQL parser's error output can be as internals-revealing as a raw DB
// error).
func validateGraphQLSDL(sdl string) error {
	if strings.TrimSpace(sdl) == "" {
		return fmt.Errorf("SDL must not be empty")
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: sdl})
	if err != nil {
		return fmt.Errorf("invalid GraphQL SDL: %w", err)
	}
	if schema.Query == nil {
		return fmt.Errorf("GraphQL SDL must define a Query type")
	}
	return nil
}
