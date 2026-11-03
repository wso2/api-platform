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

package a2ax

import (
	"fmt"
	"strings"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
)

// Task is the part of an A2A task a scenario asserts on.
type Task struct {
	ID        string
	ContextID string
	// State is the protocol's state name, such as TASK_STATE_COMPLETED.
	State string
	// Terminal reports whether the state admits no further change.
	Terminal bool
	// Artifacts holds the text of each artifact, one entry per artifact.
	Artifacts []string
}

// Card is the part of an Agent Card a scenario asserts on.
type Card struct {
	Name   string
	Skills []string
}

// PushConfig identifies one push notification config.
type PushConfig struct {
	ID     string
	TaskID string
}

// Event is one streamed event and when it was handed to the caller, measured from the start of
// the call. A response held whole and released at the end yields every event at effectively
// the same offset, which is what keeps buffering observable through the SDK.
type Event struct {
	// Type is the event kind: task, message, status-update or artifact-update.
	Type string
	// State is the task state the event carries, empty when it carries none.
	State string
	// Text is the event rendered as the text a scenario asserts on.
	Text string
	// TaskID is the task the event belongs to, empty when it names none.
	TaskID string
	// Artifact is the artifact text an artifact-update event carries.
	Artifact string
	Offset   time.Duration
}

// Outcome is the typed result of one call. Exactly one payload field is populated per call,
// or Err is set.
type Outcome struct {
	Method string
	Err    error

	Task        *Task
	Message     *string
	Tasks       []Task
	Card        *Card
	PushConfig  *PushConfig
	PushConfigs []PushConfig

	Events         []Event
	StreamDuration time.Duration
}

// TaskID returns the task the outcome concerns: the returned task's, or else the last streamed
// event that named one. It is empty when the call produced no task.
func (o *Outcome) TaskID() string {
	if o == nil {
		return ""
	}
	if o.Task != nil && o.Task.ID != "" {
		return o.Task.ID
	}
	for i := len(o.Events) - 1; i >= 0; i-- {
		if o.Events[i].TaskID != "" {
			return o.Events[i].TaskID
		}
	}
	return ""
}

// ArtifactText concatenates the text of every artifact the outcome carries, whether it arrived
// on a task or as streamed artifact-update events.
func (o *Outcome) ArtifactText() (string, error) {
	if o == nil {
		return "", fmt.Errorf("a2ax: no outcome")
	}
	var parts []string
	if o.Task != nil {
		parts = append(parts, o.Task.Artifacts...)
	}
	for _, event := range o.Events {
		if event.Type == eventArtifactUpdate {
			parts = append(parts, event.Artifact)
		}
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("%s produced no artifact (result: %s)", o.Method, o.Kind())
	}
	return strings.Join(parts, "\n"), nil
}

// FinalState returns the task state carried by the last streamed event.
func (o *Outcome) FinalState() (string, error) {
	if o == nil || len(o.Events) == 0 {
		return "", fmt.Errorf("the stream delivered no events")
	}
	last := o.Events[len(o.Events)-1]
	if last.State == "" {
		return "", fmt.Errorf("the last stream event is a %s event, which carries no task state: %s", last.Type, o.Summary())
	}
	return last.State, nil
}

// Kind names what the outcome carries, for an assertion message.
func (o *Outcome) Kind() string {
	switch {
	case o == nil:
		return "no outcome"
	case o.Task != nil:
		return "task"
	case o.Message != nil:
		return "message"
	case o.Card != nil:
		return "agent card"
	case o.PushConfig != nil:
		return "push notification config"
	case o.PushConfigs != nil:
		return "push notification config list"
	case o.Tasks != nil:
		return "task list"
	case len(o.Events) > 0:
		return fmt.Sprintf("%d stream event(s)", len(o.Events))
	default:
		return "no payload"
	}
}

// Summary renders the streamed events with their arrival offsets, for an assertion message.
func (o *Outcome) Summary() string {
	if o == nil || len(o.Events) == 0 {
		return "stream delivered no events"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d event(s) over %s:", len(o.Events), o.StreamDuration.Round(time.Millisecond))
	for i, event := range o.Events {
		fmt.Fprintf(&b, "\n  [%d] +%s %s %s", i, event.Offset.Round(time.Millisecond), event.Type, truncate(event.Text, 200))
	}
	return b.String()
}

const (
	eventTask           = "task"
	eventMessage        = "message"
	eventStatusUpdate   = "status-update"
	eventArtifactUpdate = "artifact-update"
)

func convertTask(task *a2a.Task) *Task {
	if task == nil {
		return nil
	}
	converted := &Task{
		ID: string(task.ID), ContextID: task.ContextID,
		State: string(task.Status.State), Terminal: task.Status.State.Terminal(),
		Artifacts: []string{},
	}
	for _, artifact := range task.Artifacts {
		if artifact != nil {
			converted.Artifacts = append(converted.Artifacts, partsText(artifact.Parts))
		}
	}
	return converted
}

func convertCard(card *a2a.AgentCard) *Card {
	if card == nil {
		return nil
	}
	converted := &Card{Name: card.Name, Skills: []string{}}
	for _, skill := range card.Skills {
		converted.Skills = append(converted.Skills, skill.ID)
	}
	return converted
}

func convertPushConfig(config *a2a.PushConfig) *PushConfig {
	if config == nil {
		return nil
	}
	return &PushConfig{ID: config.ID, TaskID: string(config.TaskID)}
}

func convertEvent(event a2a.Event, offset time.Duration) Event {
	converted := Event{Offset: offset}
	if event != nil {
		converted.TaskID = string(event.TaskInfo().TaskID)
	}
	switch typed := event.(type) {
	case *a2a.TaskStatusUpdateEvent:
		converted.Type = eventStatusUpdate
		converted.State = string(typed.Status.State)
		converted.Text = converted.State
		if typed.Status.Message != nil {
			converted.Text += " " + partsText(typed.Status.Message.Parts)
		}
	case *a2a.TaskArtifactUpdateEvent:
		converted.Type = eventArtifactUpdate
		if typed.Artifact != nil {
			converted.Artifact = partsText(typed.Artifact.Parts)
		}
		converted.Text = converted.Artifact
	case *a2a.Task:
		converted.Type = eventTask
		converted.State = string(typed.Status.State)
		converted.Text = converted.State
	case *a2a.Message:
		converted.Type = eventMessage
		converted.Text = partsText(typed.Parts)
	default:
		converted.Type = fmt.Sprintf("%T", event)
	}
	return converted
}

func partsText(parts a2a.ContentParts) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == nil {
			continue
		}
		if text := part.Text(); text != "" {
			texts = append(texts, text)
		}
	}
	return strings.Join(texts, "\n")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
