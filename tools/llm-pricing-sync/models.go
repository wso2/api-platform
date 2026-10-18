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

package main

// catalog is the pricing-file namespace a model is priced from. azure-llm-cost
// reads azure/ for Azure OpenAI models and azure_ai/ for Foundry's own models.
type catalog string

const (
	catalogAzureOpenAI catalog = "azure"
	catalogAzureAI     catalog = "azure_ai"
)

// modelSpec ties the Retail Prices API meters of one model to its pricing keys.
//
//	product  the meter's productName, which scopes stems such as Grok's "4.7"
//	stems    SKU name prefixes; Azure spells some models differently per tier,
//	         e.g. "5.6 sol" for standard and priority meters, "56sol" for flex
//	keys     every model name the entry is written under: the name operators
//	         map deployments to, plus dated names chat completions report
type modelSpec struct {
	catalog catalog
	product string
	stems   []string
	keys    []string
}

// longContextAt272k names, by first key, the models whose long-context meters
// start above 272k input tokens, the threshold azure-llm-cost applies. Other
// models' long-context meters are left out rather than billed from the wrong
// threshold; Grok's, for one, start lower.
var longContextAt272k = map[string]bool{
	"gpt-5.4": true, "gpt-5.4-pro": true, "gpt-5.5": true,
	"gpt-5.6-sol": true, "gpt-5.6-terra": true, "gpt-5.6-luna": true,
	"gpt-6-sol": true, "gpt-6-luna": true, "gpt-6-astra": true, "gpt-6.1-sol": true,
}

// models lists the models this tool keeps priced. Adding one is a one-line
// change; a model left out keeps whatever its pricing entries already say.
var models = []modelSpec{
	// Azure OpenAI.
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5", "5"}, []string{"gpt-5", "gpt-5-2025-08-07"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5 Mini", "5 mini"}, []string{"gpt-5-mini", "gpt-5-mini-2025-08-07"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5 Nano"}, []string{"gpt-5-nano", "gpt-5-nano-2025-08-07"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5 Chat"}, []string{"gpt-5-chat"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"gpt 5 pro"}, []string{"gpt-5-pro"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5.1", "5.1"}, []string{"gpt-5.1", "gpt-5.1-2025-11-13"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5.1 chat"}, []string{"gpt-5.1-chat", "gpt-5.1-chat-2025-11-13"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.1 codex"}, []string{"gpt-5.1-codex", "gpt-5.1-codex-2025-11-13"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.1 codex max"}, []string{"gpt-5.1-codex-max"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.1 codex mini"}, []string{"gpt-5.1-codex-mini", "gpt-5.1-codex-mini-2025-11-13"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5.2", "5.2"}, []string{"gpt-5.2", "gpt-5.2-2025-12-11"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5.2 chat"}, []string{"gpt-5.2-chat", "gpt-5.2-chat-2025-12-11"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.2 codex"}, []string{"gpt-5.2-codex"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"GPT 5.2 pro"}, []string{"gpt-5.2-pro", "gpt-5.2-pro-2025-12-11"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.3 chat"}, []string{"gpt-5.3-chat"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.3 codex"}, []string{"gpt-5.3-codex"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.4", "54"}, []string{"gpt-5.4"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.4 mini", "54 mini"}, []string{"gpt-5.4-mini"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.4 nano", "54 nano"}, []string{"gpt-5.4-nano"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.4 pro", "54 pro"}, []string{"gpt-5.4-pro"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.5", "55"}, []string{"gpt-5.5"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.6 sol", "56sol"}, []string{"gpt-5.6-sol"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.6 terra", "56terra"}, []string{"gpt-5.6-terra"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT5", []string{"5.6 luna", "56luna"}, []string{"gpt-5.6-luna"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT6", []string{"6-sol"}, []string{"gpt-6-sol"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT6", []string{"6-luna"}, []string{"gpt-6-luna"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT6", []string{"6-astra"}, []string{"gpt-6-astra"}},
	{catalogAzureOpenAI, "Azure OpenAI GPT6", []string{"6.1-sol"}, []string{"gpt-6.1-sol"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4.1"}, []string{"gpt-4.1", "gpt-4.1-2025-04-14"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4.1 mini"}, []string{"gpt-4.1-mini", "gpt-4.1-mini-2025-04-14"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4.1 nano"}, []string{"gpt-4.1-nano", "gpt-4.1-nano-2025-04-14"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4o 1120"}, []string{"gpt-4o-2024-11-20"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4o 0806"}, []string{"gpt-4o-2024-08-06"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"gpt 4o mini 0718"}, []string{"gpt-4o-mini-2024-07-18"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"o3 0416"}, []string{"o3", "o3-2025-04-16"}},
	{catalogAzureOpenAI, "Azure OpenAI", []string{"o3-pro"}, []string{"o3-pro", "o3-pro-2025-06-10"}},
	{catalogAzureOpenAI, "Azure OpenAI Reasoning", []string{"o4-mini 0416"}, []string{"o4-mini", "o4-mini-2025-04-16"}},
	{catalogAzureOpenAI, "Azure OpenAI Reasoning", []string{"codex mini"}, []string{"codex-mini"}},

	// Azure AI Foundry's own models.
	{catalogAzureAI, "Azure Deepseek Models", []string{"V3.2"}, []string{"deepseek-v3.2"}},
	{catalogAzureAI, "Azure Deepseek Models", []string{"V3.2 SP"}, []string{"deepseek-v3.2-speciale"}},
	{catalogAzureAI, "Azure Deepseek Models", []string{"V4 Pro"}, []string{"deepseek-v4-pro"}},
	{catalogAzureAI, "Azure Deepseek Models", []string{"V4 Flash"}, []string{"deepseek-v4-flash"}},
	{catalogAzureAI, "Azure Deepseek Models", []string{"V4.1 Flash"}, []string{"deepseek-v4.1-flash"}},
	{catalogAzureAI, "Azure Deepseek Models", []string{"MAI-DS-R1"}, []string{"MAI-DS-R1"}},
	{catalogAzureAI, "Azure Grok Models", []string{"Grok-4"}, []string{"grok-4"}},
	{catalogAzureAI, "Azure Grok Models", []string{"4.3"}, []string{"grok-4.3"}},
	{catalogAzureAI, "Azure Grok Models", []string{"4.6"}, []string{"grok-4.6"}},
	{catalogAzureAI, "Azure Grok Models", []string{"4.7"}, []string{"grok-4.7"}},
	{catalogAzureAI, "Azure Grok Models", []string{"Code Fast 1"}, []string{"grok-code-fast-1"}},
	{catalogAzureAI, "Azure Kimi", []string{"K2 Thinking"}, []string{"kimi-k2-thinking"}},
	{catalogAzureAI, "Azure Kimi", []string{"K2.5 Thinking", "K2.5"}, []string{"kimi-k2.5"}},
	{catalogAzureAI, "Azure Kimi", []string{"K2.6 Thinking", "K2.6"}, []string{"kimi-k2.6"}},
	{catalogAzureAI, "Azure Kimi", []string{"K2.7 Code"}, []string{"kimi-k2.7-code"}},
	{catalogAzureAI, "Azure Llama Models", []string{"Llama 3.3 70B"}, []string{"Llama-3.3-70B-Instruct"}},
	{catalogAzureAI, "Azure Llama Models", []string{"Llama 4 Maverick 17B"}, []string{"Llama-4-Maverick-17B-128E-Instruct-FP8"}},
	{catalogAzureAI, "Azure Mistral Models", []string{"Large 3"}, []string{"mistral-large-3"}},
	{catalogAzureAI, "Azure OpenAI OSS Models", []string{"gpt-oss-120B"}, []string{"gpt-oss-120b"}},
	{catalogAzureAI, "Azure Phi Models", []string{"Phi-4"}, []string{"Phi-4"}},
	{catalogAzureAI, "Azure Phi Models", []string{"Phi-4-Mini"}, []string{"Phi-4-mini-instruct"}},
	{catalogAzureAI, "Azure Phi Models", []string{"Phi-4-reasoning"}, []string{"Phi-4-reasoning"}},
	{catalogAzureAI, "Azure Phi Models", []string{"Phi-4-mini-reasoning"}, []string{"Phi-4-mini-reasoning"}},
}
