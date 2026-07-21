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

// The v1 types are the conversion hub (the CRD storage version). Each root
// kind implements the marker Hub() method from
// sigs.k8s.io/controller-runtime/pkg/conversion so that the served v1alpha1
// spoke types can be converted to and from these canonical types by the
// conversion webhook.

func (*RestApi) Hub()             {}
func (*APIGateway) Hub()          {}
func (*ApiKey) Hub()             {}
func (*APIPolicy) Hub()          {}
func (*Certificate) Hub()         {}
func (*LlmProvider) Hub()         {}
func (*LlmProviderTemplate) Hub() {}
func (*LlmProxy) Hub()            {}
func (*ManagedSecret) Hub()       {}
func (*Mcp) Hub()                 {}
func (*Subscription) Hub()        {}
func (*SubscriptionPlan) Hub()    {}
