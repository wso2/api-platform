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

// Package contentsafety provides a deterministic Azure AI Content Safety-compatible service.
//
// The text:analyze route answers like Azure for the requests the azure-content-safety-content-
// moderation policy sends, and rejects what Azure rejects:
//
//   - a missing or wrong Ocp-Apim-Subscription-Key header (SubscriptionKey) → 401;
//   - an api-version other than APIVersion, text longer than MaxTextLength characters, an unknown
//     outputType, or an unknown or repeated category → 400 with Azure's error code
//     InvalidRequestBody.
//
// Each requested category is scored from the text; no categories means all four, as in Azure:
//
//   - a severity marker "<category>-severity-<n>" (category hate, sexual, selfharm or violence;
//     n from 0 to 7) sets that category's severity to n;
//   - otherwise a category keyword in the text (for example "hate", "attack") scores
//     severityFlagged, and anything else 0.
//
// With outputType FourSeverityLevels, the default, a severity is rounded down to 0, 2, 4 or 6;
// EightSeverityLevels reports it unchanged.
//
// Text containing "simulate mixed analysis" is analysed as usual, but the categoriesAnalysis array
// starts with entries that are not objects (a string, a number, null, a boolean and an array)
// before the real ones.
//
// Text can ask for a failure instead of an analysis:
//
//   - "simulate malformed json" → 200 with a truncated JSON body;
//   - "simulate missing analysis" → 200 with valid JSON but no categoriesAnalysis;
//   - "simulate invalid analysis" → 200 with a categoriesAnalysis that is not an array;
//   - "simulate bad request" → 400 InvalidRequestBody;
//   - "simulate forbidden" → 403 Forbidden;
//   - both "simulate" and "error" → 500.
package contentsafety
