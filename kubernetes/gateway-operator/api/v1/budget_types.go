/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The types below mirror the gateway-controller management-API schemas
// BudgetSpec / BudgetLimit / BudgetSplit / BudgetFilter / BudgetExemption and the
// labels of BudgetMetadata (api/management-openapi.yaml). The two representations are
// separately maintained with no compile-time link, so a change to either side must be
// mirrored in the other.
//
// Only structural constraints from the management-API schema are expressed here. The
// semantic rules (which filter types may repeat, an agent filter's project, CEL
// expressions that compile, a split by project on a project budget, ...) are enforced by
// the gateway-controller at deploy time and surface on this CR's Programmed condition.

// BudgetSplit gives each value of a type its own limit.
type BudgetSplit struct {
	// Type is what gets its own limit: each LLM provider, project, agent or API key, or
	// each value of an attribute expression.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=llmProvider;project;agent;apiKey;attribute
	Type string `json:"type"`

	// Expression is a CEL expression over the request returning a string or a list of
	// strings. attribute only.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Expression *string `json:"expression,omitempty"`
}

// BudgetLimit is how much may be spent per period, what happens when it is used up,
// and what gets its own limit. Amounts are in USD.
type BudgetLimit struct {
	// Amount is a positive amount in USD with up to 2 decimal places, such as "100.00".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([1-9][0-9]{0,11}(\.[0-9]{1,2})?|0\.([1-9][0-9]?|0[1-9]))$`
	Amount string `json:"amount"`

	// Period is a UTC calendar day, an ISO week starting Monday, or a calendar month.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=day;week;month
	Period string `json:"period"`

	// Action: block refuses requests at the block point; allow only counts.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=block;allow
	Action string `json:"action"`

	// BlockAt is the percentage of the amount at which a block budget refuses requests.
	// Defaults to 100.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	BlockAt *int32 `json:"blockAt,omitempty"`

	// SplitBy gives each value of a type its own limit. Omitted means one shared limit
	// for all matching usage.
	// +optional
	SplitBy *BudgetSplit `json:"splitBy,omitempty"`
}

// BudgetFilter is one condition on the LLM usage the budget counts. Filters combine
// with AND.
type BudgetFilter struct {
	// Type is what the filter matches.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=llmProvider;project;agent;apiKey;attribute
	Type string `json:"type"`

	// Id is the LLM provider, project, agent or API key name. Required for llmProvider;
	// omitted for project, agent or apiKey means any one. Not for an attribute.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	Id *string `json:"id,omitempty"`

	// Project is the agent's project, for an agent filter with an id: agent names are
	// unique only per project.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Project *string `json:"project,omitempty"`

	// Expression is a CEL condition over the request returning bool. attribute only.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Expression *string `json:"expression,omitempty"`
}

// BudgetExemption is traffic left out of the budget: an attribute condition, a project,
// an agent, an LLM provider or an API key.
type BudgetExemption struct {
	// Type is what the exemption matches.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=attribute;project;agent;llmProvider;apiKey
	Type string `json:"type"`

	// Id is the project, agent, LLM provider or API key name.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Id *string `json:"id,omitempty"`

	// Project is the agent's project. agent only.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Project *string `json:"project,omitempty"`

	// Expression is a CEL condition over the request. attribute only.
	// +optional
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1024
	Expression *string `json:"expression,omitempty"`
}

// BudgetWarnings is the control plane's warning settings. The gateway ignores it.
type BudgetWarnings struct {
	// Thresholds are the percentages of the limit at which the control plane warns.
	// +optional
	Thresholds []int32 `json:"thresholds,omitempty"`

	// Channels are the notification channels the control plane warns.
	// +optional
	Channels []string `json:"channels,omitempty"`
}

// BudgetConfigData mirrors the management-API BudgetSpec, plus the budget's labels
// (metadata.labels of the management-API kind, which are a list of words; the
// Kubernetes metadata.labels stay a map and select the gateway).
type BudgetConfigData struct {
	// Labels are one-word labels to organize and find budgets. Not used to enforce; kept
	// with the budget and pushed to the control plane with it. At most 10, no repeats;
	// each 1-63 characters of letters, digits, '.', '_' or '-', starting and ending with a
	// letter or digit.
	// +optional
	// +kubebuilder:validation:MaxItems=10
	// +listType=set
	Labels []BudgetLabel `json:"labels,omitempty"`

	// Status: an inactive budget is kept but matches no traffic. Defaults to active.
	// +optional
	// +kubebuilder:validation:Enum=active;inactive
	Status *string `json:"status,omitempty"`

	// Limit is how much may be spent per period and what happens when it is used up.
	// +kubebuilder:validation:Required
	Limit BudgetLimit `json:"limit"`

	// Filters are the LLM usage the budget counts. Every filter must match (AND). Empty
	// means all LLM usage.
	// +optional
	// +kubebuilder:validation:MaxItems=20
	Filters []BudgetFilter `json:"filters,omitempty"`

	// Exemptions are matching traffic left out of this budget.
	// +optional
	// +kubebuilder:validation:MaxItems=100
	Exemptions []BudgetExemption `json:"exemptions,omitempty"`

	// DisplayName is accepted so a control plane Budget manifest applies unchanged. It is
	// control-plane only: the operator does not send it, and the gateway ignores it.
	// +optional
	DisplayName *string `json:"displayName,omitempty"`

	// Description is accepted so a control plane Budget manifest applies unchanged. It is
	// control-plane only: the operator does not send it, and the gateway ignores it.
	// +optional
	Description *string `json:"description,omitempty"`

	// Warnings is accepted so a control plane Budget manifest applies unchanged. It is
	// control-plane only: the operator does not send it, and the gateway ignores it.
	// +optional
	Warnings *BudgetWarnings `json:"warnings,omitempty"`
}

// BudgetLabel is one budget label.
// +kubebuilder:validation:MaxLength=63
// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?$`
type BudgetLabel string

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:resource:path=budgets,singular=budget

// Budget is the Schema for the budgets API: a spending limit on LLM usage, enforced by
// the gateway.
type Budget struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the Budget configuration.
	// +kubebuilder:validation:Required
	Spec   BudgetConfigData `json:"spec"`
	Status ResourceStatus   `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BudgetList contains a list of Budget.
type BudgetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Budget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Budget{}, &BudgetList{})
}
