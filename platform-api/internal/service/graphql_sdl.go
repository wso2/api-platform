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
	"errors"
	"strings"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	"github.com/wso2/api-platform/platform-api/api"
)

// validateGraphQLSDL parses and validates a directly-supplied GraphQL SDL
// document, returning every issue found — nil when the SDL is valid. It
// rejects malformed SDL and SDL with no Query type.
//
// Unlike a schemaSource url/introspection failure, these issues are safe to
// return to the caller verbatim (see ValidateGraphQLSchemaResponse.sdlErrors'
// doc comment in openapi.yaml): they describe the caller's own submitted
// text, not an upstream/network failure whose detail could map internal
// topology (error-handling.md, ssrf-prevention.md).
func validateGraphQLSDL(sdl string) []api.GraphQLSdlValidationIssue {
	if strings.TrimSpace(sdl) == "" {
		return []api.GraphQLSdlValidationIssue{{Message: "SDL must not be empty"}}
	}
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "schema.graphql", Input: sdl})
	if err != nil {
		return sdlIssuesFromError(err)
	}
	if schema.Query == nil {
		return []api.GraphQLSdlValidationIssue{{Message: "GraphQL SDL must define a Query type"}}
	}
	return nil
}

// sdlIssuesFromError extracts the line/column gqlparser anchors its error to,
// when it can. Schema loading fails fast on the first problem found, so
// there is always exactly one issue here, never a batch.
func sdlIssuesFromError(err error) []api.GraphQLSdlValidationIssue {
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) {
		issue := api.GraphQLSdlValidationIssue{Message: gqlErr.Message}
		if len(gqlErr.Locations) > 0 {
			line, column := gqlErr.Locations[0].Line, gqlErr.Locations[0].Column
			issue.Line, issue.Column = &line, &column
		}
		return []api.GraphQLSdlValidationIssue{issue}
	}
	return []api.GraphQLSdlValidationIssue{{Message: err.Error()}}
}
