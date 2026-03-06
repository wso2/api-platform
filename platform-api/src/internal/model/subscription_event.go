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

package model

// SubscriptionCreatedEvent represents the payload for a subscription.created event.
type SubscriptionCreatedEvent struct {
	ApiId          string `json:"apiId"`
	SubscriptionId string `json:"subscriptionId"`
	ApplicationId  string `json:"applicationId"`
	Status         string `json:"status"`
}

// SubscriptionUpdatedEvent represents the payload for a subscription.updated event.
type SubscriptionUpdatedEvent struct {
	ApiId          string `json:"apiId"`
	SubscriptionId string `json:"subscriptionId"`
	ApplicationId  string `json:"applicationId"`
	Status         string `json:"status"`
}

// SubscriptionDeletedEvent represents the payload for a subscription.deleted event.
type SubscriptionDeletedEvent struct {
	ApiId          string `json:"apiId"`
	SubscriptionId string `json:"subscriptionId"`
	ApplicationId  string `json:"applicationId"`
}

