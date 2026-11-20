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

package deploy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
)

const (
	throughNewVersion = "through the new API version"
	asGraphQLAPI      = "as a GraphQL API"
)

// malformedBody is a request body that is not a JSON document.
const malformedBody = `{"environment": `

// RegisterBuildSteps binds the steps that act on the builds a scenario saw by the order it saw
// them in: the build a deployment rendered or a build it prepared.
func (s *Steps) RegisterBuildSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I prepare (a|\d+) builds? of the synthetic REST API$`, s.prepareBuilds)
	sc.Step(`^I deploy build (\d+) of the (synthetic REST API|synthetic GraphQL API) to the active gateway$`, s.deployBuild)
	sc.Step(`^I delete build (\d+) of the synthetic REST API(?: (`+throughNewVersion+`|`+asGraphQLAPI+`))?$`, s.deleteBuild)
	sc.Step(`^the synthetic REST API builds should (include|not include) build (\d+)$`, s.assertBuildPresence)
	sc.Step(`^the synthetic REST API builds should be exactly builds ((?:\d+, )*\d+ and \d+)$`, s.assertBuildsExactly)
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
	ref := cloudbase.APIRefFor(label)
	buildID, err := seenBuild(ctx, ref, n)
	if err != nil {
		return err
	}
	deployed, err := s.DeployRefToActiveGateway(ctx, ref, buildID)
	if err != nil {
		return err
	}
	if deployed.Build != buildID {
		return fmt.Errorf("the deployment runs build %q, want build %d (%s)", deployed.Build, n, buildID)
	}
	return nil
}

// deleteBuild asks to delete the nth build the scenario saw for the synthetic REST API, through
// the API itself or the other way named, and leaves the answer for the scenario.
func (s *Steps) deleteBuild(ctx context.Context, n int, through string) error {
	buildID, err := seenBuild(ctx, cloudbase.PrimaryAPI, n)
	if err != nil {
		return err
	}
	ref, kind := cloudbase.PrimaryAPI, ""
	switch through {
	case "":
	case throughNewVersion:
		ref = cloudbase.VersionAPI
	case asGraphQLAPI:
		kind = cloudbase.KindGraphQLAPI
	default:
		return fmt.Errorf("unsupported way to address a build: %q", through)
	}
	scope, err := cloudbase.RefScopePath(ctx, ref)
	if err != nil {
		return err
	}
	_, err = s.DoAuthenticated(ctx, http.MethodDelete, cloudbase.WithKind(scope+"/builds/"+url.PathEscape(buildID), kind), nil)
	return err
}

// assertBuildPresence waits until the synthetic REST API's builds include, or no longer include,
// the nth build the scenario saw.
func (s *Steps) assertBuildPresence(ctx context.Context, presence string, n int) error {
	buildID, err := seenBuild(ctx, cloudbase.PrimaryAPI, n)
	if err != nil {
		return err
	}
	want := presence == "include"
	return retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		s.FetchBuilds,
		func(items []map[string]any) bool {
			_, listed := buildPositions(items)[buildID]
			return listed == want
		}, fmt.Sprintf("waiting for the builds to %s build %d (%s)", presence, n, buildID))
}

// assertBuildsExactly waits until the synthetic REST API's builds are exactly the builds the
// scenario saw that the list names, newest first, as "6, 5, 4, 3 and 1" names them.
func (s *Steps) assertBuildsExactly(ctx context.Context, list string) error {
	want, err := buildsNamed(ctx, list)
	if err != nil {
		return err
	}
	var listed []string
	err = retry.Await(ctx, retry.Options{Timeout: cloudbase.PropagationTimeout(s.Topo), Interval: 3 * time.Second},
		s.FetchBuilds,
		func(items []map[string]any) bool {
			listed = listed[:0]
			for _, item := range items {
				id, _ := item["buildId"].(string)
				listed = append(listed, id)
			}
			return slices.Equal(listed, want)
		}, "waiting for the builds to be exactly "+strings.Join(want, ", "))
	if err != nil {
		return fmt.Errorf("%w; listed %v", err, listed)
	}
	return nil
}

// buildsNamed resolves a list such as "6, 5, 4, 3 and 1" to the builds the scenario saw.
func buildsNamed(ctx context.Context, list string) ([]string, error) {
	var ids []string
	for _, part := range strings.FieldsFunc(strings.ReplaceAll(list, " and ", ","), func(r rune) bool { return r == ',' }) {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("%q is not a list of build numbers", list)
		}
		id, err := seenBuild(ctx, cloudbase.PrimaryAPI, n)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// assertBuildsDistinct checks that no two builds the scenario saw share an identifier.
func (s *Steps) assertBuildsDistinct(ctx context.Context) error {
	builds, err := cloudbase.SeenBuilds(ctx, cloudbase.PrimaryAPI)
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
	scope, err := cloudbase.APIScopePath(ctx)
	if err != nil {
		return err
	}
	path := scope + "/builds"
	if request == "deployment" {
		path = scope + "/deployments"
	}
	_, err = s.DoAuthenticatedContent(ctx, http.MethodPost, path, "application/json", []byte(malformedBody))
	return err
}
