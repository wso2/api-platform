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

package agentproto

// A2A protocol version 1.0: eleven canonical operations, each with one
// HTTP+JSON route.
//
// Transcribed from the vendored protocol definition at
// gateway/gateway-controller/specification/a2a/v1.0/a2a.proto and the
// specification's method-mapping reference (section 5.3). The tests in that
// directory check this table against the proto.
//
// This file is only ever edited to correct 1.0. A later protocol version gets
// its own file and its own vendored proto directory, because an Agent selects
// one version and the two tables are not interchangeable.

var v1_0Operations = []Operation{
	SendMessage,
	SendStreamingMessage,
	GetTask,
	ListTasks,
	CancelTask,
	SubscribeToTask,
	CreateTaskPushNotificationConfig,
	GetTaskPushNotificationConfig,
	ListTaskPushNotificationConfigs,
	DeleteTaskPushNotificationConfig,
	GetExtendedAgentCard,
}

// v1_0Bindings gives each 1.0 operation its HTTP+JSON routes.
//
// SubscribeToTask follows the specification document (POST), not the proto
// (GET). Upstream disagrees with itself at the pinned commit; see the SOURCE
// file beside the vendored proto. The consequence is real and one-directional:
// a client generated from the proto sends GET and gets a 404. Serving both
// verbs would be one operation with two entries here and still one policy
// chain — the slice shape exists for exactly that case.
var v1_0Bindings = map[Operation][]HTTPBinding{
	SendMessage:                      {{Method: "POST", PathTemplate: "/message:send"}},
	SendStreamingMessage:             {{Method: "POST", PathTemplate: "/message:stream"}},
	GetTask:                          {{Method: "GET", PathTemplate: "/tasks/{id}"}},
	ListTasks:                        {{Method: "GET", PathTemplate: "/tasks"}},
	CancelTask:                       {{Method: "POST", PathTemplate: "/tasks/{id}:cancel"}},
	SubscribeToTask:                  {{Method: "POST", PathTemplate: "/tasks/{id}:subscribe"}},
	CreateTaskPushNotificationConfig: {{Method: "POST", PathTemplate: "/tasks/{id}/pushNotificationConfigs"}},
	GetTaskPushNotificationConfig:    {{Method: "GET", PathTemplate: "/tasks/{id}/pushNotificationConfigs/{configId}"}},
	ListTaskPushNotificationConfigs:  {{Method: "GET", PathTemplate: "/tasks/{id}/pushNotificationConfigs"}},
	DeleteTaskPushNotificationConfig: {{Method: "DELETE", PathTemplate: "/tasks/{id}/pushNotificationConfigs/{configId}"}},
	GetExtendedAgentCard:             {{Method: "GET", PathTemplate: "/extendedAgentCard"}},
}

var v1_0Table = versionTable{
	operations: v1_0Operations,
	bindings:   v1_0Bindings,
}
