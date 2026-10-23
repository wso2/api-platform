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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
)

const (
	// configReadyTimeout bounds how long a new gateway's configuration may take to become readable.
	configReadyTimeout = 3 * time.Minute
	// configSaveTimeout bounds how long a setup write is retried while the platform reports a
	// concurrent change to the gateway's configuration.
	configSaveTimeout = 2 * time.Minute
	// concurrentWriteTimeout bounds how long rounds of simultaneous writes are tried before one
	// is rejected as a conflict.
	concurrentWriteTimeout = 90 * time.Second
	// concurrentWritePath is the setting the simultaneous writers each give a value of their own.
	concurrentWritePath = "gateway.config.policy_configurations.ratelimit_v1.memory.max_entries"
	// concurrentWriteBase is the lowest value a simultaneous writer sends; the setting accepts
	// 1000 to 1000000.
	concurrentWriteBase = 10000
)

// configPhases are the phases a configuration reports.
var configPhases = map[string]bool{"applying": true, "healthy": true, "failed": true, "unknown": true}

// editableTypes are the types a configuration setting can have.
var editableTypes = map[string]bool{"enum": true, "boolean": true, "integer": true, "duration": true, "quantity": true, "string": true}

// configAttempt is one write of a single setting, with what the platform answered.
type configAttempt struct {
	Path    string `json:"path"`
	Want    string `json:"want,omitempty"`
	Status  int    `json:"status"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// parseConfigCell reads a table cell as the JSON value it spells (a number, a boolean, null or a
// quoted string) and otherwise as plain text, so a scenario can send a value of any type.
func parseConfigCell(raw string) any {
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &value); err == nil {
		return value
	}
	return raw
}

// configValuesFromTable reads a path-and-value table into the values of a configuration patch.
func configValuesFromTable(table *godog.Table) (map[string]any, error) {
	raw, err := tableValues(table)
	if err != nil {
		return nil, err
	}
	values := make(map[string]any, len(raw))
	for path, cell := range raw {
		values[path] = parseConfigCell(cell)
	}
	return values, nil
}

// attemptSpecs reads a table with a path and a value column and an optional message column.
func attemptSpecs(table *godog.Table) ([]configAttempt, []any, error) {
	if table == nil || len(table.Rows) < 2 {
		return nil, nil, errors.New("the table needs a header row and at least one row")
	}
	header := table.Rows[0]
	if len(header.Cells) < 2 || len(header.Cells) > 3 ||
		strings.TrimSpace(header.Cells[0].Value) != "path" || strings.TrimSpace(header.Cells[1].Value) != "value" ||
		(len(header.Cells) == 3 && strings.TrimSpace(header.Cells[2].Value) != "message") {
		return nil, nil, errors.New(`the header row must be "path", "value" and optionally "message"`)
	}
	var specs []configAttempt
	var values []any
	for _, row := range table.Rows[1:] {
		if len(row.Cells) != len(header.Cells) {
			return nil, nil, fmt.Errorf("a row has %d cells, the header has %d", len(row.Cells), len(header.Cells))
		}
		spec := configAttempt{Path: strings.TrimSpace(row.Cells[0].Value)}
		if spec.Path == "" {
			return nil, nil, errors.New("a row names no path")
		}
		if len(row.Cells) == 3 {
			spec.Want = strings.TrimSpace(row.Cells[2].Value)
		}
		specs = append(specs, spec)
		values = append(values, parseConfigCell(row.Cells[1].Value))
	}
	return specs, values, nil
}

// errorFields reads the code and message of an error envelope; both are empty for any other body.
func errorFields(body []byte) (code, message string) {
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return "", ""
	}
	return envelope.Code, envelope.Message
}

// judgeRejection reports why an attempt was not refused as an invalid request that names its
// setting; it is empty when the attempt was refused as it should be.
func judgeRejection(attempt configAttempt) string {
	switch {
	case attempt.Status != http.StatusBadRequest:
		return fmt.Sprintf("answered %d, want 400 (%s)", attempt.Status, attempt.Message)
	case attempt.Code != "invalid_request":
		return fmt.Sprintf("answered error code %q, want invalid_request", attempt.Code)
	case !strings.Contains(attempt.Message, attempt.Path):
		return fmt.Sprintf("refused without naming the setting: %q", attempt.Message)
	case attempt.Want != "" && !strings.Contains(attempt.Message, attempt.Want):
		return fmt.Sprintf("refused with %q, want a message containing %q", attempt.Message, attempt.Want)
	}
	return ""
}

// configurationTarget resolves the gateway a configuration step acts on.
func configurationTarget(ctx context.Context, label string) (string, error) {
	key := keyManagedGatewayID
	if label == "active" {
		key = keyGatewayID
	}
	return requiredValue(ctx, key)
}

// configurationPath is the route of a gateway's configuration.
func configurationPath(id string) string {
	return "/managed-gateways/" + url.PathEscape(id) + "/configuration"
}

// readGatewayConfiguration reads a gateway's configuration and publishes the response.
func (s *Steps) readGatewayConfiguration(ctx context.Context, label string) error {
	id, err := configurationTarget(ctx, label)
	if err != nil {
		return err
	}
	if err := tcontext.Set(ctx, keyConfigGatewayID, id); err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodGet, configurationPath(id), nil)
	return err
}

// awaitConfigurationReadable waits until the synthetic gateway's configuration can be read: the
// release it lives on is created while the gateway is provisioned, and a write is judged against
// it.
func (s *Steps) awaitConfigurationReadable(ctx context.Context) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: configReadyTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, configurationPath(id), nil)
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK {
				return nil, retry.Transient(responseError(resp))
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil },
		"waiting for the configuration of managed gateway "+id+" to be readable")
}

// validateConfigurationDocument checks a configuration response is what the API describes: the
// gateway's identity, only editable settings with a value, the catalogue of what may be changed
// with its bounds, the rules between settings, and a reconcile phase.
func validateConfigurationDocument(body []byte, wantID string) error {
	var document struct {
		ID          string           `json:"id"`
		Environment string           `json:"environment"`
		Values      map[string]any   `json:"values"`
		Editable    []map[string]any `json:"editable"`
		Constraints []map[string]any `json:"constraints"`
		Status      struct {
			Phase              string `json:"phase"`
			LastTransitionTime string `json:"lastTransitionTime"`
		} `json:"status"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("decoding the configuration: %w", err)
	}
	if document.ID != wantID {
		return fmt.Errorf("the configuration is of gateway %q, want %q", document.ID, wantID)
	}
	if strings.TrimSpace(document.Environment) == "" {
		return errors.New("the configuration names no environment")
	}
	if document.Values == nil {
		return errors.New("the configuration has no values object")
	}
	if !configPhases[document.Status.Phase] {
		return fmt.Errorf("the configuration reports phase %q, want applying, healthy, failed or unknown", document.Status.Phase)
	}
	if stamp := document.Status.LastTransitionTime; stamp != "" {
		if _, err := time.Parse(time.RFC3339, stamp); err != nil {
			return fmt.Errorf("lastTransitionTime %q is not a date-time", stamp)
		}
	}
	if len(document.Editable) == 0 {
		return errors.New("the configuration lists no editable setting")
	}
	paths := map[string]bool{}
	for _, entry := range document.Editable {
		path, err := validateEditableEntry(entry)
		if err != nil {
			return err
		}
		if paths[path] {
			return fmt.Errorf("editable setting %q is listed twice", path)
		}
		paths[path] = true
	}
	for path, value := range document.Values {
		if !paths[path] {
			return fmt.Errorf("the configuration reports %q, which is not an editable setting", path)
		}
		if value == nil {
			return fmt.Errorf("the configuration reports %q as null; a setting running its default is left out", path)
		}
	}
	for _, rule := range document.Constraints {
		if rule["type"] != "notGreaterThan" {
			return fmt.Errorf("constraint of unknown type %v", rule["type"])
		}
		for _, side := range []string{"path", "than"} {
			if name, _ := rule[side].(string); !paths[name] {
				return fmt.Errorf("a constraint %s %q is not an editable setting", side, name)
			}
		}
	}
	return nil
}

// validateEditableEntry checks one entry of the editable catalogue and returns its path.
func validateEditableEntry(entry map[string]any) (string, error) {
	path, _ := entry["path"].(string)
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("an editable setting has no path: %v", entry)
	}
	kind, _ := entry["type"].(string)
	if !editableTypes[kind] {
		return "", fmt.Errorf("editable setting %q has unknown type %q", path, kind)
	}
	has := func(field string) bool {
		text, ok := entry[field].(string)
		return ok && strings.TrimSpace(text) != ""
	}
	switch kind {
	case "enum":
		if values, _ := entry["values"].([]any); len(values) == 0 {
			return "", fmt.Errorf("enum setting %q lists no values", path)
		}
	case "integer", "duration", "quantity":
		if !has("min") || !has("max") {
			return "", fmt.Errorf("%s setting %q must carry both min and max", kind, path)
		}
	case "string":
		if !has("max") {
			return "", fmt.Errorf("string setting %q must carry a max length", path)
		}
	}
	return path, nil
}

func (s *Steps) assertConfigurationDocument(ctx context.Context) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return responseError(response)
	}
	id, err := requiredValue(ctx, keyConfigGatewayID)
	if err != nil {
		return err
	}
	if err := validateConfigurationDocument(response.Body, id); err != nil {
		return fmt.Errorf("%w (%s)", err, response.Describe())
	}
	return nil
}

// writeGatewayConfiguration writes the settings of a table to the synthetic gateway and
// publishes the response without requiring success.
func (s *Steps) writeGatewayConfiguration(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	values, err := configValuesFromTable(table)
	if err != nil {
		return err
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, configurationPath(id), map[string]any{"values": values})
	return err
}

// saveGatewayConfiguration writes settings that the scenario depends on. A write racing the
// provisioning of the gateway is answered with a conflict, which is retried; any other refusal
// fails the step.
func (s *Steps) saveGatewayConfiguration(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	values, err := configValuesFromTable(table)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: configSaveTimeout, Interval: 3 * time.Second},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := s.doAuthenticated(ctx, http.MethodPut, configurationPath(id), map[string]any{"values": values})
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
				return nil, responseError(resp)
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil && resp.StatusCode == http.StatusOK },
		"waiting for the configuration of managed gateway "+id+" to be saved")
}

// writeLongString writes a string of the given length to one setting and publishes the response.
func (s *Steps) writeLongString(ctx context.Context, path string, length int) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	if length < 1 {
		return fmt.Errorf("the string length must be positive, got %d", length)
	}
	_, err = s.doAuthenticated(ctx, http.MethodPut, configurationPath(id),
		map[string]any{"values": map[string]any{path: strings.Repeat("a", length)}})
	return err
}

// writeEachValue writes every setting of a table in a write of its own, so one refusal cannot hide
// another, and records what the platform answered to each.
func (s *Steps) writeEachValue(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	specs, values, err := attemptSpecs(table)
	if err != nil {
		return err
	}
	for index := range specs {
		resp, err := s.doAuthenticated(ctx, http.MethodPut, configurationPath(id),
			map[string]any{"values": map[string]any{specs[index].Path: values[index]}})
		if err != nil {
			return err
		}
		specs[index].Status = resp.StatusCode
		specs[index].Code, specs[index].Message = errorFields(resp.Body)
	}
	raw, err := json.Marshal(specs)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, keyConfigAttempts, string(raw))
}

// assertEveryWriteRejected checks every recorded write was refused as an invalid request that
// names its setting, and lists all that were not.
func (s *Steps) assertEveryWriteRejected(ctx context.Context) error {
	var attempts []configAttempt
	if err := json.Unmarshal([]byte(optionalValue(ctx, keyConfigAttempts)), &attempts); err != nil || len(attempts) == 0 {
		return errors.New("no gateway configuration writes were recorded")
	}
	var failures []string
	for _, attempt := range attempts {
		if why := judgeRejection(attempt); why != "" {
			failures = append(failures, attempt.Path+": "+why)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("%d of %d writes were not refused as invalid:\n  %s", len(failures), len(attempts), strings.Join(failures, "\n  "))
	}
	return nil
}

func (s *Steps) assertConfigurationPhase(ctx context.Context, want string) error {
	response, err := httpx.Published(ctx)
	if err != nil {
		return err
	}
	var document struct {
		Status struct {
			Phase string `json:"phase"`
		} `json:"status"`
	}
	if err := json.Unmarshal(response.Body, &document); err != nil {
		return fmt.Errorf("decoding the configuration: %w (%s)", err, response.Describe())
	}
	if document.Status.Phase != want {
		return fmt.Errorf("the configuration phase is %q, want %q (%s)", document.Status.Phase, want, response.Describe())
	}
	return nil
}

// configurationValues reads the settings the synthetic gateway currently holds.
func (s *Steps) configurationValues(ctx context.Context, id string) (map[string]any, error) {
	resp, err := s.doAuthenticatedRaw(ctx, http.MethodGet, configurationPath(id), nil)
	if err != nil {
		return nil, retry.Transient(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, retry.Transient(responseError(resp))
	}
	var document struct {
		Values map[string]any `json:"values"`
	}
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil, retry.Transient(fmt.Errorf("decoding the configuration: %w", err))
	}
	return document.Values, nil
}

// assertConfigurationReports waits until a read of the configuration shows the table's values.
func (s *Steps) assertConfigurationReports(ctx context.Context, table *godog.Table) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	expected, err := configValuesFromTable(table)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (string, error) {
			values, err := s.configurationValues(ctx, id)
			if err != nil {
				return "", err
			}
			var differing []string
			for path, want := range expected {
				if got, present := values[path]; !present || !sameValue(got, want) {
					differing = append(differing, fmt.Sprintf("%s is %v, want %v", path, got, want))
				}
			}
			sort.Strings(differing)
			return strings.Join(differing, "; "), nil
		},
		func(differing string) bool { return differing == "" },
		"waiting for the configuration of managed gateway "+id+" to report the written values")
}

// assertConfigurationOmits waits until a read of the configuration no longer reports a setting.
func (s *Steps) assertConfigurationOmits(ctx context.Context, path string) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	return retry.Await(ctx, retry.Options{Timeout: propagationTimeout(s.topo), Interval: 3 * time.Second},
		func(ctx context.Context) (map[string]any, error) { return s.configurationValues(ctx, id) },
		func(values map[string]any) bool { _, present := values[path]; return values != nil && !present },
		"waiting for the configuration of managed gateway "+id+" to stop reporting "+path)
}

// writeRound is what one round of simultaneous writes was answered.
type writeRound struct {
	statuses []int
	conflict *httpx.Response
	unwanted []string
}

// judgeRound sorts the answers of one round: a write is accepted or rejected as a conflict, and
// anything else is a fault.
func judgeRound(responses []*httpx.Response) writeRound {
	round := writeRound{}
	for index, resp := range responses {
		round.statuses = append(round.statuses, resp.StatusCode)
		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusConflict:
			if round.conflict == nil {
				round.conflict = resp
			}
			if code, _ := errorFields(resp.Body); code != "conflict" {
				round.unwanted = append(round.unwanted, fmt.Sprintf("writer %d: conflict with error code %q", index, code))
			}
		default:
			round.unwanted = append(round.unwanted, fmt.Sprintf("writer %d: %s", index, resp.Describe()))
		}
	}
	return round
}

// writeConcurrently has several writers change the same setting at once, round after round, until
// one of them is refused as a conflict. A platform that lets every write through would lose all
// but the last, silently.
func (s *Steps) writeConcurrently(ctx context.Context, writers int) error {
	id, err := requiredValue(ctx, keyManagedGatewayID)
	if err != nil {
		return err
	}
	if writers < 2 {
		return fmt.Errorf("at least two writers are needed, got %d", writers)
	}
	token, err := bearerToken(ctx)
	if err != nil {
		return err
	}
	var rounds []string
	var rejected *httpx.Response
	roundNumber := 0
	err = retry.Await(ctx, retry.Options{Timeout: concurrentWriteTimeout, Interval: time.Second},
		func(ctx context.Context) (writeRound, error) {
			roundNumber++
			responses := make([]*httpx.Response, writers)
			failures := make([]error, writers)
			var group sync.WaitGroup
			for writer := range writers {
				group.Add(1)
				go func() {
					defer group.Done()
					value := concurrentWriteBase + roundNumber*writers + writer
					responses[writer], failures[writer] = s.sendConfigurationWrite(ctx, token, id, value)
				}()
			}
			group.Wait()
			if err := errors.Join(failures...); err != nil {
				return writeRound{}, retry.Transient(err)
			}
			round := judgeRound(responses)
			rounds = append(rounds, fmt.Sprint(round.statuses))
			if len(round.unwanted) > 0 {
				return writeRound{}, fmt.Errorf("round %d: %s", roundNumber, strings.Join(round.unwanted, "; "))
			}
			if round.conflict != nil {
				rejected = round.conflict
			}
			return round, nil
		},
		func(round writeRound) bool { return round.conflict != nil },
		"waiting for one of "+fmt.Sprint(writers)+" simultaneous writes to be rejected as a conflict")
	if err != nil {
		return fmt.Errorf("%w (answers per round: %s)", err, strings.Join(rounds, " "))
	}
	if s.funnel == nil {
		return nil
	}
	return s.funnel.Publish(ctx, rejected)
}

// sendConfigurationWrite sends one writer's value without publishing the response.
func (s *Steps) sendConfigurationWrite(ctx context.Context, token, id string, value int) (*httpx.Response, error) {
	raw, err := json.Marshal(map[string]any{"values": map[string]any{concurrentWritePath: value}})
	if err != nil {
		return nil, err
	}
	base, err := s.apiURL(configurationPath(id))
	if err != nil {
		return nil, err
	}
	return s.client.Do(ctx, httpx.Request{
		Method:  http.MethodPut,
		URL:     base,
		Headers: map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"},
		Body:    raw,
	}, 0, 0)
}
