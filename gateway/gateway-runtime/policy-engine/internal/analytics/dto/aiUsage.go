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

package dto

// AIUsage carries the token counts the provider reported for an LLM call. A nil
// count was not reported, which is not the same as zero: a streamed response
// without usage, or an embeddings call with no completion count, must not reach a
// sink that prices tokens as a call that used none.
//
// AITokenUsage stays as it is because its JSON shape is published as event
// metadata; this type is kept separate so new counts can be added without changing
// that shape.
type AIUsage struct {
	PromptTokens     *int
	CompletionTokens *int
	TotalTokens      *int
}
