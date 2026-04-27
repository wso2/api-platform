/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

// Subscription represents subscription attributes in an analytics event.
type Subscription struct {
	BillingCustomerID     string `json:"billingCustomerId"`
	BillingSubscriptionID string `json:"billingSubscriptionId"`
	Status                string `json:"status"`
	PlanName              string `json:"planName"`
}

// GetBillingCustomerID returns the billing customer ID.
func (s *Subscription) GetBillingCustomerID() string {
	return s.BillingCustomerID
}

// SetBillingCustomerID sets the billing customer ID.
func (s *Subscription) SetBillingCustomerID(billingCustomerID string) {
	s.BillingCustomerID = billingCustomerID
}

// GetBillingSubscriptionID returns the billing subscription ID.
func (s *Subscription) GetBillingSubscriptionID() string {
	return s.BillingSubscriptionID
}

// SetBillingSubscriptionID sets the billing subscription ID.
func (s *Subscription) SetBillingSubscriptionID(billingSubscriptionID string) {
	s.BillingSubscriptionID = billingSubscriptionID
}

// GetStatus returns the subscription status.
func (s *Subscription) GetStatus() string {
	return s.Status
}

// SetStatus sets the subscription status.
func (s *Subscription) SetStatus(status string) {
	s.Status = status
}

// GetPlanName returns the subscription plan name.
func (s *Subscription) GetPlanName() string {
	return s.PlanName
}

// SetPlanName sets the subscription plan name.
func (s *Subscription) SetPlanName(planName string) {
	s.PlanName = planName
}
