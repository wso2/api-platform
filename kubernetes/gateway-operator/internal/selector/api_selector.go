/*
Copyright 2025.

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

package selector

import (
	"context"

	apiv1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// APISelector handles API selection logic for gateways
type APISelector struct {
	client client.Client
}

// NewAPISelector creates a new API selector
func NewAPISelector(client client.Client) *APISelector {
	return &APISelector{
		client: client,
	}
}

// SelectAPIsForGateway returns all APIs that should be deployed to the given gateway
func (s *APISelector) SelectAPIsForGateway(ctx context.Context, gateway *apiv1.APIGateway) ([]apiv1.RestApi, error) {
	switch gateway.Spec.APISelector.Scope {
	case apiv1.ClusterScope:
		return s.selectClusterScopedAPIs(ctx, gateway)
	case apiv1.NamespacedScope:
		return s.selectNamespacedAPIs(ctx, gateway)
	case apiv1.LabelSelectorScope:
		return s.selectLabelBasedAPIs(ctx, gateway)
	default:
		// Default to cluster scope
		return s.selectClusterScopedAPIs(ctx, gateway)
	}
}

// selectClusterScopedAPIs selects all APIs from all namespaces
func (s *APISelector) selectClusterScopedAPIs(ctx context.Context, gateway *apiv1.APIGateway) ([]apiv1.RestApi, error) {
	apiList := &apiv1.RestApiList{}

	// List all APIs across all namespaces
	if err := s.client.List(ctx, apiList); err != nil {
		return nil, err
	}

	// Filter APIs that explicitly reference this gateway or have no gateway reference
	return apiList.Items, nil
}

// selectNamespacedAPIs selects APIs from specific namespaces
func (s *APISelector) selectNamespacedAPIs(ctx context.Context, gateway *apiv1.APIGateway) ([]apiv1.RestApi, error) {
	var allAPIs []apiv1.RestApi

	// If no namespaces specified, use gateway's own namespace
	namespaces := gateway.Spec.APISelector.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{gateway.Namespace}
	}

	// List APIs from each specified namespace
	for _, ns := range namespaces {
		apiList := &apiv1.RestApiList{}
		if err := s.client.List(ctx, apiList, client.InNamespace(ns)); err != nil {
			return nil, err
		}
		allAPIs = append(allAPIs, apiList.Items...)
	}

	// Filter APIs that explicitly reference this gateway or have no gateway reference
	return allAPIs, nil
}

// selectLabelBasedAPIs selects APIs based on label selectors
func (s *APISelector) selectLabelBasedAPIs(ctx context.Context, gateway *apiv1.APIGateway) ([]apiv1.RestApi, error) {
	// Build label selector from matchLabels and matchExpressions
	selector := labels.NewSelector()

	// Add matchLabels
	for key, value := range gateway.Spec.APISelector.MatchLabels {
		req, err := labels.NewRequirement(key, "=", []string{value})
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*req)
	}

	// Add matchExpressions
	for _, expr := range gateway.Spec.APISelector.MatchExpressions {
		var op selection.Operator
		switch expr.Operator {
		case metav1.LabelSelectorOpIn:
			op = selection.In
		case metav1.LabelSelectorOpNotIn:
			op = selection.NotIn
		case metav1.LabelSelectorOpExists:
			op = selection.Exists
		case metav1.LabelSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		default:
			continue
		}

		req, err := labels.NewRequirement(expr.Key, op, expr.Values)
		if err != nil {
			return nil, err
		}
		selector = selector.Add(*req)
	}

	// List APIs with label selector
	apiList := &apiv1.RestApiList{}
	listOpts := &client.ListOptions{
		LabelSelector: selector,
	}

	if err := s.client.List(ctx, apiList, listOpts); err != nil {
		return nil, err
	}

	// Filter APIs that explicitly reference this gateway or have no gateway reference
	return apiList.Items, nil
}

// IsAPISelectedByGateway checks if a specific API is selected by a gateway
func (s *APISelector) IsAPISelectedByGateway(ctx context.Context, api *apiv1.RestApi, gateway *apiv1.APIGateway) (bool, error) {

	// Otherwise check based on gateway's selector
	switch gateway.Spec.APISelector.Scope {
	case apiv1.ClusterScope:
		// Cluster scope accepts all APIs
		return true, nil

	case apiv1.NamespacedScope:
		// Check if API is in one of the selected namespaces
		namespaces := gateway.Spec.APISelector.Namespaces
		if len(namespaces) == 0 {
			namespaces = []string{gateway.Namespace}
		}
		for _, ns := range namespaces {
			if api.Namespace == ns {
				return true, nil
			}
		}
		return false, nil

	case apiv1.LabelSelectorScope:
		// Check if API labels match the selector
		apiLabels := labels.Set(api.Labels)

		// Check matchLabels
		for key, value := range gateway.Spec.APISelector.MatchLabels {
			if apiLabels.Get(key) != value {
				return false, nil
			}
		}

		// Check matchExpressions
		for _, expr := range gateway.Spec.APISelector.MatchExpressions {
			matches := false
			apiValue := apiLabels.Get(expr.Key)

			switch expr.Operator {
			case metav1.LabelSelectorOpIn:
				for _, v := range expr.Values {
					if apiValue == v {
						matches = true
						break
					}
				}
			case metav1.LabelSelectorOpNotIn:
				matches = true
				for _, v := range expr.Values {
					if apiValue == v {
						matches = false
						break
					}
				}
			case metav1.LabelSelectorOpExists:
				_, matches = api.Labels[expr.Key]
			case metav1.LabelSelectorOpDoesNotExist:
				_, exists := api.Labels[expr.Key]
				matches = !exists
			}

			if !matches {
				return false, nil
			}
		}

		return true, nil

	default:
		// Default to cluster scope
		return true, nil
	}
}
