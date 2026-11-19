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

package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/cucumber/godog"
	"gopkg.in/yaml.v3"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	"github.com/wso2/api-platform/tests/framework/suites/cloud/steps/cloudbase"
	"github.com/wso2/api-platform/tests/framework/testbench/services/specs"
)

const (
	importOpenAPIPath   = cloudbase.RestAPIsCollection + "/import-openapi"
	validateOpenAPIPath = cloudbase.RestAPIsCollection + "/validate-openapi"
	// Form values that upload a document instead of sending text: a testbench specs fixture by
	// name, or a generated OpenAPI document of a given size in bytes.
	specFieldPrefix          = "@spec:"
	generatedSpecFieldPrefix = "@generated-spec:"
	// operationProbeHeader is the request header an operation's header policy sets.
	operationProbeHeader = "x-apip-operation-probe"
	operationProbeValue  = "kept"
	headerPolicyName     = "set-headers"
	keyValidatedFixture  = "cloudValidatedSpecFixture"
)

// importFormKeys lists the fields an import values table may carry.
var importFormKeys = map[string]bool{"file": true, "url": true, "description": true}

// definitionKeys are the fields of a REST API answer that make up the definition it is updated with.
var definitionKeys = []string{
	"id", "displayName", "description", "context", "version", "projectId",
	"upstream", "operations", "policies", "security",
}

// openAPIMethods are the keys of an OpenAPI path item that name an operation.
var openAPIMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// RegisterAPIDefinitionSteps binds the steps that create a REST API from an OpenAPI definition,
// replace and validate definitions, and check what the platform keeps of them.
func (s *Steps) RegisterAPIDefinitionSteps(sc cloudbase.StepRegistrar) {
	sc.Step(`^I import a synthetic REST API from the spec fixture "([^"]*)"$`, s.importAPI)
	sc.Step(`^I import a new version "([^"]*)" of the synthetic REST API from the spec fixture "([^"]*)"$`, s.importNewAPIVersion)
	sc.Step(`^I import the deleted synthetic REST API again from the spec fixture "([^"]*)"$`, s.reimportAPI)
	sc.Step(`^I submit a synthetic REST API import with:$`, s.submitAPIImport)
	sc.Step(`^I validate the spec fixture "([^"]*)"$`, s.validateSpecFixture)
	sc.Step(`^the OpenAPI validation should (list the errors and return the definition|find no errors)$`, s.assertValidation)
	sc.Step(`^I replace the definition of the synthetic REST API with the spec fixture "([^"]*)"$`, s.replaceDefinition)
	sc.Step(`^I add a header policy to the "([^"]*)" operation of the synthetic REST API$`, s.addOperationPolicy)
	sc.Step(`^I read the synthetic REST API$`, s.readAPI)
	sc.Step(`^the (synthetic REST API|new API version) should have the operations of the spec fixture "([^"]*)"$`, s.assertOperationsOfFixture)
	sc.Step(`^the "([^"]*)" operation of the synthetic REST API should (keep|not have) the header policy$`, s.assertOperationPolicy)
	sc.Step(`^the stored definition of the (synthetic REST API|new API version) should be the spec fixture "([^"]*)"$`, s.assertStoredDefinition)
	sc.Step(`^the synthetic REST API should have no stored definition$`, s.assertNoStoredDefinition)
	sc.Step(`^the submitted synthetic REST API should (exist|not exist)$`, s.assertSubmittedAPI)
	sc.Step(`^the new API version should have the synthetic REST API's display name and the version "([^"]*)"$`, s.assertNewVersionOfTheAPI)
	sc.Step(`^the synthetic REST API should have an operation named "([^"]*)"$`, s.assertOperationNamed)
	sc.Step(`^the echoed request should carry the header policy's header$`, s.assertProbeHeaderEchoed)
	sc.Step(`^the echoed request should not carry the header policy's header$`, s.assertProbeHeaderAbsent)
}

// documentPart resolves a form value to the document it uploads. isDocument is false for a value
// that is sent as text.
func documentPart(value string) (name string, content []byte, isDocument bool, err error) {
	switch {
	case strings.HasPrefix(value, specFieldPrefix):
		name = strings.TrimPrefix(value, specFieldPrefix)
		content, err = specs.Fixture(name)
		return name, content, true, err
	case strings.HasPrefix(value, generatedSpecFieldPrefix):
		size, convErr := strconv.Atoi(strings.TrimPrefix(value, generatedSpecFieldPrefix))
		if convErr != nil || size <= 0 {
			return "", nil, true, fmt.Errorf("%q does not name a document size in bytes", value)
		}
		content, err = specs.Generate(size)
		return "generated.yaml", content, true, err
	default:
		return "", nil, false, nil
	}
}

// documentForm builds a multipart form of text fields and uploaded documents, in the order given.
func documentForm(fields [][2]string) ([]byte, string, error) {
	body := &bytes.Buffer{}
	form := multipart.NewWriter(body)
	for _, field := range fields {
		name, content, isDocument, err := documentPart(field[1])
		if err != nil {
			return nil, "", fmt.Errorf("form field %q: %w", field[0], err)
		}
		if !isDocument {
			if err := form.WriteField(field[0], field[1]); err != nil {
				return nil, "", err
			}
			continue
		}
		part, err := form.CreateFormFile(field[0], name)
		if err != nil {
			return nil, "", err
		}
		if _, err := part.Write(content); err != nil {
			return nil, "", err
		}
	}
	if err := form.Close(); err != nil {
		return nil, "", err
	}
	return body.Bytes(), form.FormDataContentType(), nil
}

// importDetails are the details an import gives the REST API it creates.
type importDetails struct {
	id, displayName, version, context string
}

// importFields are the form fields that describe the synthetic REST API an import creates, with
// the testbench backend as its upstream.
func importFields(ctx context.Context, details importDetails) ([][2]string, error) {
	projectID, err := cloudbase.RequiredValue(ctx, cloudbase.KeyProjectID)
	if err != nil {
		return nil, err
	}
	backend, err := cloudbase.TestbenchURL(cloudbase.BackendEnv, cloudbase.DefaultBackend)
	if err != nil {
		return nil, err
	}
	upstream, err := json.Marshal(map[string]any{"main": map[string]string{"url": backend}})
	if err != nil {
		return nil, err
	}
	return [][2]string{
		{"id", details.id}, {"displayName", details.displayName}, {"version", details.version},
		{"context", details.context}, {"projectId", projectID}, {"upstream", string(upstream)},
	}, nil
}

// definitionOf keeps the fields of a REST API answer that make up its definition.
func definitionOf(resp *httpx.Response) (map[string]any, error) {
	var answer map[string]any
	if err := json.Unmarshal(resp.Body, &answer); err != nil {
		return nil, fmt.Errorf("decoding the REST API: %w", err)
	}
	definition := map[string]any{}
	for _, key := range definitionKeys {
		if value, present := answer[key]; present {
			definition[key] = value
		}
	}
	return definition, nil
}

func (s *Steps) importAPI(ctx context.Context, fixture string) error {
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-api", cloudbase.PrimaryAPI.ID); err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, cloudbase.PrimaryAPI.ID)
	if err != nil {
		return err
	}
	return s.importAPIAs(ctx, cloudbase.PrimaryAPI, importDetails{
		id: id, displayName: "Synthetic " + id, version: "1.0.0", context: "/" + id + "/v1",
	}, fixture)
}

// importNewAPIVersion imports a second API under the first API's display name with another
// version, which is how the platform models a new version of an API.
func (s *Steps) importNewAPIVersion(ctx context.Context, version, fixture string) error {
	definition, err := s.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	displayName, _ := definition["displayName"].(string)
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-api", cloudbase.VersionAPI.ID); err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, cloudbase.VersionAPI.ID)
	if err != nil {
		return err
	}
	return s.importAPIAs(ctx, cloudbase.VersionAPI, importDetails{
		id: id, displayName: displayName, version: version, context: "/" + id + "/" + version,
	}, fixture)
}

// reimportAPI imports the synthetic REST API under the id, name, version and context it had
// before it was deleted.
func (s *Steps) reimportAPI(ctx context.Context, fixture string) error {
	definition, err := s.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	details := importDetails{}
	for field, target := range map[string]*string{
		"id": &details.id, "displayName": &details.displayName, "version": &details.version, "context": &details.context,
	} {
		value, _ := definition[field].(string)
		if value == "" {
			return fmt.Errorf("the deleted synthetic REST API has no %s to import again", field)
		}
		*target = value
	}
	if err := s.importAPIAs(ctx, cloudbase.PrimaryAPI, details, fixture); err != nil {
		return err
	}
	// The builds seen for the deleted API are not the new API's, though it has the same id.
	if err := tcontext.Set(ctx, cloudbase.PrimaryAPI.BuildsOwner, ""); err != nil {
		return err
	}
	return cloudbase.OwnBuildState(ctx)
}

// importAPIAs creates the REST API a ref names from a specs fixture and keeps its definition. It
// is registered for cleanup before the request and stays registered whatever the answer, since an
// import can fail after the API is stored; cleanup treats an absent API as deleted.
func (s *Steps) importAPIAs(ctx context.Context, ref cloudbase.APIRef, details importDetails, fixture string) error {
	if err := cloudbase.RegisterAPIForCleanup(ctx, details.id); err != nil {
		return err
	}
	fields, err := importFields(ctx, details)
	if err != nil {
		return err
	}
	fields = append(fields, [2]string{"description", cloudbase.SyntheticDescription}, [2]string{"file", specFieldPrefix + fixture})
	body, contentType, err := documentForm(fields)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPost, importOpenAPIPath, contentType, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return cloudbase.ResponseError(resp)
	}
	definition, err := definitionOf(resp)
	if err != nil {
		return err
	}
	createdID, err := s.keepCreatedAPI(ctx, details.id, definition)
	if err != nil {
		return err
	}
	if err := cloudbase.StoreDefinition(ctx, ref, definition); err != nil {
		return err
	}
	return tcontext.Set(ctx, ref.ID, createdID)
}

// submitAPIImport sends an import whose definition source comes from a values table and leaves
// the answer for the scenario, so it can assert on a refusal. The API stays registered for cleanup
// whatever the answer, and an API created under another id is registered under that id.
func (s *Steps) submitAPIImport(ctx context.Context, table *godog.Table) error {
	values, err := cloudbase.TableValues(table)
	if err != nil {
		return err
	}
	if err := cloudbase.GenerateResourceAndStore(ctx, "syn-api", cloudbase.KeyRejectedAPIID); err != nil {
		return err
	}
	id, err := cloudbase.RequiredValue(ctx, cloudbase.KeyRejectedAPIID)
	if err != nil {
		return err
	}
	fields, err := importFields(ctx, importDetails{
		id: id, displayName: "Synthetic " + id, version: "1.0.0", context: "/" + id + "/v1",
	})
	if err != nil {
		return err
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		if !importFormKeys[key] {
			return fmt.Errorf("unsupported import value %q", key)
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := cloudbase.ExpandCloudValue(ctx, values[key])
		if err != nil {
			return err
		}
		fields = append(fields, [2]string{key, value})
	}
	body, contentType, err := documentForm(fields)
	if err != nil {
		return err
	}
	if err := cloudbase.RegisterAPIForCleanup(ctx, id); err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPost, importOpenAPIPath, contentType, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return nil
	}
	definition, err := definitionOf(resp)
	if err != nil {
		return err
	}
	createdID, err := s.keepCreatedAPI(ctx, id, definition)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, cloudbase.KeyRejectedAPIID, createdID)
}

// keepCreatedAPI returns the id the platform created an API under. When it is not the id the
// request named, the created API is registered for cleanup in place of the requested one, and is
// deleted at once if it cannot be registered.
func (s *Steps) keepCreatedAPI(ctx context.Context, requested string, created map[string]any) (string, error) {
	createdID, _ := created["id"].(string)
	if createdID == "" {
		return "", fmt.Errorf("the created REST API has no id")
	}
	if createdID == requested {
		return createdID, nil
	}
	if err := cloudbase.RegisterAPIForCleanup(ctx, createdID); err != nil {
		if _, deleteErr := s.DoAuthenticatedRaw(ctx, http.MethodDelete, cloudbase.RestAPIsCollection+"/"+url.PathEscape(createdID), nil); deleteErr != nil {
			return "", fmt.Errorf("%w; deleting it: %v", err, deleteErr)
		}
		return "", err
	}
	cloudbase.Deregister(ctx, cleanup.KindAPI, requested)
	return createdID, nil
}

func (s *Steps) validateSpecFixture(ctx context.Context, fixture string) error {
	if err := tcontext.Set(ctx, keyValidatedFixture, fixture); err != nil {
		return err
	}
	body, contentType, err := documentForm([][2]string{{"file", specFieldPrefix + fixture}})
	if err != nil {
		return err
	}
	_, err = s.DoAuthenticatedContent(ctx, http.MethodPost, validateOpenAPIPath, contentType, body)
	return err
}

// assertValidation checks the validation answer for the fixture last validated: an invalid one
// lists its errors and returns the bytes that were validated, a valid one lists none.
func (s *Steps) assertValidation(ctx context.Context, outcome string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(response)
	}
	var answer struct {
		IsValid *bool `json:"isValid"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
		Content *string `json:"content"`
	}
	if err := json.Unmarshal(response.Body, &answer); err != nil || answer.IsValid == nil {
		return fmt.Errorf("the answer is not a validation result: %s", response.Describe())
	}
	if outcome == "find no errors" {
		if !*answer.IsValid || len(answer.Errors) > 0 {
			return fmt.Errorf("the validation found errors in a valid definition: %s", response.Describe())
		}
		return nil
	}
	if *answer.IsValid || len(answer.Errors) == 0 {
		return fmt.Errorf("the validation found no errors in an invalid definition: %s", response.Describe())
	}
	for i, problem := range answer.Errors {
		if strings.TrimSpace(problem.Message) == "" {
			return fmt.Errorf("validation error %d has no message: %s", i+1, response.Describe())
		}
	}
	fixture, err := cloudbase.RequiredValue(ctx, keyValidatedFixture)
	if err != nil {
		return err
	}
	want, err := specs.Fixture(fixture)
	if err != nil {
		return err
	}
	if answer.Content == nil || *answer.Content != string(want) {
		return fmt.Errorf("the validation did not return the definition it validated (%s)", fixture)
	}
	return nil
}

// replaceDefinition uploads a specs fixture as the synthetic REST API's definition and keeps the
// definition the API has afterwards.
func (s *Steps) replaceDefinition(ctx context.Context, fixture string) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.PrimaryAPI.ID)
	if err != nil {
		return err
	}
	body, contentType, err := documentForm([][2]string{{"file", specFieldPrefix + fixture}})
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedContent(ctx, http.MethodPut,
		cloudbase.RestAPIsCollection+"/"+url.PathEscape(id)+"/openapi", contentType, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	return s.refreshDefinition(ctx, cloudbase.PrimaryAPI)
}

// refreshDefinition keeps the definition the platform holds now for the API a ref points at.
func (s *Steps) refreshDefinition(ctx context.Context, ref cloudbase.APIRef) error {
	definition, err := s.readDefinitionOf(ctx, ref)
	if err != nil {
		return err
	}
	return cloudbase.StoreDefinition(ctx, ref, definition)
}

// readAPIOf reads the API a ref points at without publishing the answer.
func (s *Steps) readAPIOf(ctx context.Context, ref cloudbase.APIRef) (*httpx.Response, error) {
	id, err := cloudbase.RequiredValue(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, ref.Collection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, cloudbase.ResponseError(resp)
	}
	return resp, nil
}

// headerPolicy is the policy that makes an operation set the probe header on the request it
// sends upstream.
func headerPolicy() map[string]any {
	return map[string]any{
		"name": headerPolicyName, "version": "v1",
		"params": map[string]any{"request": map[string]any{"headers": []any{
			map[string]any{"name": operationProbeHeader, "value": operationProbeValue},
		}}},
	}
}

// operationNamed finds an operation of a definition by its name.
func operationNamed(definition map[string]any, name string) (map[string]any, error) {
	operations, _ := definition["operations"].([]any)
	for _, raw := range operations {
		operation, _ := raw.(map[string]any)
		if operation["name"] == name {
			return operation, nil
		}
	}
	return nil, fmt.Errorf("the REST API has no operation named %q", name)
}

// addOperationPolicy gives one operation of the synthetic REST API the header policy.
func (s *Steps) addOperationPolicy(ctx context.Context, name string) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.PrimaryAPI.ID)
	if err != nil {
		return err
	}
	definition, err := s.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	operation, err := operationNamed(definition, name)
	if err != nil {
		return err
	}
	request, _ := operation["request"].(map[string]any)
	if request == nil {
		return fmt.Errorf("operation %q has no request", name)
	}
	policies, _ := request["policies"].([]any)
	probe := headerPolicy()
	if !slices.ContainsFunc(policies, func(policy any) bool { return reflect.DeepEqual(normalizedJSON(policy), normalizedJSON(probe)) }) {
		request["policies"] = append(policies, probe)
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodPut, cloudbase.RestAPIsCollection+"/"+url.PathEscape(id), definition)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	return cloudbase.StoreDefinition(ctx, cloudbase.PrimaryAPI, definition)
}

// readAPI reads the synthetic REST API and publishes the answer for the scenario to assert on.
func (s *Steps) readAPI(ctx context.Context) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.PrimaryAPI.ID)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticated(ctx, http.MethodGet, cloudbase.RestAPIsCollection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	return nil
}

// fixtureOperations lists the operations a specs fixture defines as "name METHOD path".
func fixtureOperations(name string) ([]string, error) {
	content, err := specs.Fixture(name)
	if err != nil {
		return nil, err
	}
	var document struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(content, &document); err != nil {
		return nil, fmt.Errorf("decoding spec fixture %q: %w", name, err)
	}
	var operations []string
	for path, item := range document.Paths {
		for method, raw := range item {
			if !slices.Contains(openAPIMethods, method) {
				continue
			}
			operation, _ := raw.(map[string]any)
			id, _ := operation["operationId"].(string)
			operations = append(operations, id+" "+strings.ToUpper(method)+" "+path)
		}
	}
	sort.Strings(operations)
	return operations, nil
}

// apiOperations lists the operations of a REST API answer as "name METHOD path".
func apiOperations(resp *httpx.Response) ([]string, error) {
	var answer struct {
		Operations []struct {
			Name    string `json:"name"`
			Request struct {
				Method string `json:"method"`
				Path   string `json:"path"`
			} `json:"request"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil {
		return nil, fmt.Errorf("decoding the REST API: %w", err)
	}
	operations := make([]string, 0, len(answer.Operations))
	for _, operation := range answer.Operations {
		operations = append(operations,
			operation.Name+" "+strings.ToUpper(operation.Request.Method)+" "+operation.Request.Path)
	}
	sort.Strings(operations)
	return operations, nil
}

// assertOperationsOfFixture checks that the labelled API has exactly the operations a specs
// fixture defines, by name, method and path.
func (s *Steps) assertOperationsOfFixture(ctx context.Context, label, fixture string) error {
	want, err := fixtureOperations(fixture)
	if err != nil {
		return err
	}
	resp, err := s.readAPIOf(ctx, cloudbase.APIRefFor(label))
	if err != nil {
		return err
	}
	got, err := apiOperations(resp)
	if err != nil {
		return err
	}
	if !slices.Equal(got, want) {
		return fmt.Errorf("the %s has operations %v, the spec fixture %q defines %v", label, got, fixture, want)
	}
	return nil
}

// assertOperationPolicy checks whether an operation of the synthetic REST API carries the header
// policy.
func (s *Steps) assertOperationPolicy(ctx context.Context, name, want string) error {
	definition, err := s.readDefinitionOf(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	operation, err := operationNamed(definition, name)
	if err != nil {
		return err
	}
	request, _ := operation["request"].(map[string]any)
	policies, _ := request["policies"].([]any)
	carries := false
	for _, raw := range policies {
		if policy, _ := raw.(map[string]any); policy["name"] == headerPolicyName {
			carries = true
		}
	}
	if carries != (want == "keep") {
		return fmt.Errorf("operation %q has policies %v, want it to %s the header policy", name, policies, want)
	}
	return nil
}

// storedDefinitionOf reads the OpenAPI definition the platform stores for the API a ref points at.
func (s *Steps) storedDefinitionOf(ctx context.Context, ref cloudbase.APIRef) (*httpx.Response, error) {
	id, err := cloudbase.RequiredValue(ctx, ref.ID)
	if err != nil {
		return nil, err
	}
	return s.DoAuthenticatedRaw(ctx, http.MethodGet, ref.Collection+"/"+url.PathEscape(id)+"/openapi", nil)
}

// assertStoredDefinition checks that the labelled API stores a specs fixture byte for byte, as the
// platform keeps an uploaded definition unchanged.
func (s *Steps) assertStoredDefinition(ctx context.Context, label, fixture string) error {
	want, err := specs.Fixture(fixture)
	if err != nil {
		return err
	}
	resp, err := s.storedDefinitionOf(ctx, cloudbase.APIRefFor(label))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return cloudbase.ResponseError(resp)
	}
	var answer struct {
		Content *string `json:"content"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil || answer.Content == nil {
		return fmt.Errorf("the stored definition has no content: %s", resp.Describe())
	}
	if *answer.Content != string(want) {
		return fmt.Errorf("the stored definition of the %s differs from the spec fixture %q (%d bytes, want %d)",
			label, fixture, len(*answer.Content), len(want))
	}
	return nil
}

func (s *Steps) assertNoStoredDefinition(ctx context.Context) error {
	resp, err := s.storedDefinitionOf(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("expected no stored definition, got %s", resp.Describe())
	}
	return nil
}

// assertSubmittedAPI checks that the API of the last submitted import or creation exists and is
// read back under its id, or that a refused one left no API behind.
func (s *Steps) assertSubmittedAPI(ctx context.Context, presence string) error {
	id, err := cloudbase.RequiredValue(ctx, cloudbase.KeyRejectedAPIID)
	if err != nil {
		return err
	}
	resp, err := s.DoAuthenticatedRaw(ctx, http.MethodGet, cloudbase.RestAPIsCollection+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	if presence == "not exist" {
		if resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("expected REST API %s not to exist, got %s", id, resp.Describe())
		}
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected REST API %s to exist, got %s", id, resp.Describe())
	}
	if got, _ := cloudbase.StringField(resp, "id"); got != id {
		return fmt.Errorf("reading REST API %s returned the API %q", id, got)
	}
	return nil
}

// assertNewVersionOfTheAPI checks that the new API version is read back under the first API's
// display name with the given version, and the first API with another version.
func (s *Steps) assertNewVersionOfTheAPI(ctx context.Context, version string) error {
	first, err := s.readDefinitionOf(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	second, err := s.readDefinitionOf(ctx, cloudbase.VersionAPI)
	if err != nil {
		return err
	}
	name, _ := first["displayName"].(string)
	if name == "" || second["displayName"] != name {
		return fmt.Errorf("the new API version is named %v, the synthetic REST API %q", second["displayName"], name)
	}
	if second["version"] != version {
		return fmt.Errorf("the new API version has the version %v, want %q", second["version"], version)
	}
	imported, err := s.StoredDefinition(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	want, _ := imported["version"].(string)
	if want == "" || want == version || first["version"] != want {
		return fmt.Errorf("the synthetic REST API has the version %v, want the version %q it was imported with", first["version"], want)
	}
	return nil
}

// normalizedJSON is a value as it reads after a JSON round trip, so a policy built here compares
// equal to the same policy read back from the platform.
func normalizedJSON(value any) any {
	encoded, err := json.Marshal(value)
	if err != nil {
		return value
	}
	var decoded any
	if json.Unmarshal(encoded, &decoded) != nil {
		return value
	}
	return decoded
}

// readDefinitionOf reads the definition the platform holds for the API a ref points at.
func (s *Steps) readDefinitionOf(ctx context.Context, ref cloudbase.APIRef) (map[string]any, error) {
	resp, err := s.readAPIOf(ctx, ref)
	if err != nil {
		return nil, err
	}
	return definitionOf(resp)
}

// assertOperationNamed checks that the synthetic REST API the platform holds has an operation of
// the given name.
func (s *Steps) assertOperationNamed(ctx context.Context, name string) error {
	definition, err := s.readDefinitionOf(ctx, cloudbase.PrimaryAPI)
	if err != nil {
		return err
	}
	_, err = operationNamed(definition, name)
	return err
}

// assertProbeHeaderEchoed checks that the backend received the header the header policy sets.
func (s *Steps) assertProbeHeaderEchoed(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	got, present := cloudbase.EchoedHeader(response.Body, operationProbeHeader)
	if !present || got != operationProbeValue {
		return fmt.Errorf("the backend received header %s %q, want %q: %s",
			operationProbeHeader, got, operationProbeValue, response.Describe())
	}
	return nil
}

// assertProbeHeaderAbsent checks that the backend received an echoable request without the
// header the header policy sets.
func (s *Steps) assertProbeHeaderAbsent(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var echoed struct {
		Headers map[string]any `json:"headers"`
	}
	if err := json.Unmarshal(response.Body, &echoed); err != nil || echoed.Headers == nil {
		return fmt.Errorf("the response is not an echo of the request: %s", response.Describe())
	}
	for name, value := range echoed.Headers {
		if strings.EqualFold(name, operationProbeHeader) {
			return fmt.Errorf("the backend received header %s %v, want none", operationProbeHeader, value)
		}
	}
	return nil
}
