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

// Package jev provides a partitioned TypeSafe Jev System One API with deterministic answers,
// failure modes and a per-partition request log.
//
// A Jev policy appends /v1/systemone to its base URL, so a scenario points the policy at
//
//	http://testbench:3015/<partition>/<mode>
//
// where <partition> is a scenario-unique name and <mode> selects the service's behaviour: ok,
// error, slow, ratelimit-once, overloaded-once, ratelimit-always, invalid-json or missing-answer.
//
// Answers default to a low noul probability, a zero score and evenly spread choice options. A
// marker in the text Jev is asked about sets a question's answer:
//
//	jev:<key>=0.95                       a noul probability
//	jev:<key>=2@0.5                      a score and its confidence
//	jev:<key>=code@0.9                   a choice, the rest spread over the other options
//	jev:<key>=a:0.35,b:0.35,c:0.3        a choice distribution
//
// Every request is recorded, including those answered with an error, and GET
// /<partition>/test/requests reports them with their count.
package jev
