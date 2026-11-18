/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudconsole

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// Ways a step addresses a build of the synthetic REST API other than through its own API.
const (
	throughNewVersion = "through the new API version"
	asGraphQLAPI      = "as a GraphQL API"
)

// malformedBody is a request body that is not a JSON document.
const malformedBody = `{"environment": `

// registerBuildSteps binds the steps that act on the builds a scenario saw by the order it saw
// them in: the build a deployment rendered or a build it prepared.
func (s *Steps) registerBuildSteps(sc stepRegistrar) {
	sc.Step(`^I prepare (a|\d+) builds? of the synthetic REST API$`, s.prepareBuilds)
	sc.Step(`^I deploy build (\d+) of the (synthetic REST API|synthetic GraphQL API) to the active gateway$`, s.deployBuild)
	sc.Step(`^I delete build (\d+) of the synthetic REST API(?: (`+throughNewVersion+`|`+asGraphQLAPI+`))?$`, s.deleteBuild)
	sc.Step(`^the synthetic REST API builds should (include|not include) build (\d+)$`, s.assertBuildPresence)
	sc.Step(`^the synthetic REST API builds should list build (\d+) first$`, s.assertBuildListedFirst)
	sc.Step(`^the builds of the synthetic REST API should all have different identifiers$`, s.assertBuildsDistinct)
	sc.Step(`^I send a (build|deployment) request for the synthetic REST API whose body is not JSON$`, s.sendMalformedRequest)
}

// prepareBuilds prepares one build of the synthetic REST API, or the given number of them.
func (s *Steps) prepareBuilds(ctx context.Context, count string) error {
	n := 1
	if count != "a" {
		parsed, err := strconv.Atoi(count)
		if err != nil || parsed < 1 {
			return fmt.Errorf("%q is not a number of builds", count)
		}
		n = parsed
	}
	for range n {
		if err := s.prepareBuild(ctx, nil); err != nil {
			return err
		}
	}
	return nil
}

// deployBuild deploys the nth build the scenario saw for the labelled API to the active gateway
// and checks that the deployment runs that build.
func (s *Steps) deployBuild(ctx context.Context, n int, label string) error {
	ref := apiRefFor(label)
	buildID, err := seenBuild(ctx, ref, n)
	if err != nil {
		return err
	}
	deployed, err := s.deployRefToActiveGateway(ctx, ref, buildID)
	if err != nil {
		return err
	}
	if deployed.build != buildID {
		return fmt.Errorf("the deployment runs build %q, want build %d (%s)", deployed.build, n, buildID)
	}
	return nil
}

// deleteBuild asks to delete the nth build the scenario saw for the synthetic REST API, through
// the API itself or the other way named, and leaves the answer for the scenario.
func (s *Steps) deleteBuild(ctx context.Context, n int, through string) error {
	buildID, err := seenBuild(ctx, primaryAPI, n)
	if err != nil {
		return err
	}
	ref, kind := primaryAPI, ""
	switch through {
	case "":
	case throughNewVersion:
		ref = versionAPI
	case asGraphQLAPI:
		kind = kindGraphQLAPI
	default:
		return fmt.Errorf("unsupported way to address a build: %q", through)
	}
	scope, err := refScopePath(ctx, ref)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodDelete, withKind(scope+"/builds/"+url.PathEscape(buildID), kind), nil)
	return err
}

// assertBuildPresence waits until the synthetic REST API's builds include, or no longer include,
// the nth build the scenario saw.
func (s *Steps) assertBuildPresence(ctx context.Context, presence string, n int) error {
	buildID, err := seenBuild(ctx, primaryAPI, n)
	if err != nil {
		return err
	}
	want := presence == "include"
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		s.fetchBuilds,
		func(items []map[string]any) bool {
			_, listed := buildPositions(items)[buildID]
			return listed == want
		}, fmt.Sprintf("waiting for the builds to %s build %d (%s)", presence, n, buildID))
}

// assertBuildListedFirst waits until the nth build the scenario saw is the newest one listed.
func (s *Steps) assertBuildListedFirst(ctx context.Context, n int) error {
	buildID, err := seenBuild(ctx, primaryAPI, n)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		s.fetchBuilds,
		func(items []map[string]any) bool { return len(items) > 0 && items[0]["buildId"] == buildID },
		fmt.Sprintf("waiting for build %d (%s) to be listed first", n, buildID))
}

// assertBuildsDistinct checks that no two builds the scenario saw share an identifier.
func (s *Steps) assertBuildsDistinct(ctx context.Context) error {
	builds, err := seenBuilds(ctx, primaryAPI)
	if err != nil {
		return err
	}
	if len(builds) < 2 {
		return fmt.Errorf("comparing identifiers needs at least two builds, have %d", len(builds))
	}
	first := map[string]int{}
	for i, id := range builds {
		if earlier, seen := first[id]; seen {
			return fmt.Errorf("builds %d and %d share the identifier %q", earlier+1, i+1, id)
		}
		first[id] = i
	}
	return nil
}

// sendMalformedRequest sends a build or deployment request for the synthetic REST API whose body
// is not JSON, and leaves the answer for the scenario.
func (s *Steps) sendMalformedRequest(ctx context.Context, request string) error {
	scope, err := apiScopePath(ctx)
	if err != nil {
		return err
	}
	path := scope + "/builds"
	if request == "deployment" {
		path = scope + "/deployments"
	}
	_, err = s.doAuthenticatedContent(ctx, http.MethodPost, path, "application/json", []byte(malformedBody))
	return err
}
