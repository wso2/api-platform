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

// DefaultFaultEvent represents the default fault event data.
type DefaultFaultEvent struct {
	RequestTimestamp         string `json:"requestTimestamp"`
	CorrelationID            string `json:"correlationID"`
	KeyType                  string `json:"keyType"`
	ErrorType                string `json:"errorType"`
	ErrorCode                int    `json:"errorCode"`
	ErrorMessage             string `json:"errorMessage"`
	APIID                    string `json:"apiId"`
	APIType                  string `json:"apiType"`
	APIName                  string `json:"apiName"`
	APIVersion               string `json:"apiVersion"`
	APIMethod                string `json:"apiMethod"`
	APICreator               string `json:"apiCreator"`
	APICreatorTenantDomain   string `json:"apiCreatorTenantDomain"`
	ApplicationID            string `json:"applicationID"`
	ApplicationName          string `json:"applicationName"`
	ApplicationOwner         string `json:"applicationOwner"`
	RegionID                 string `json:"regionID"`
	GatewayType              string `json:"gatewayType"`
	OrganizationID           string `json:"organizationID"`
	EnvironmentID            string `json:"environmentID"`
	ProxyResponseCode        int    `json:"proxyResponseCode"`
	TargetResponseCode       int    `json:"targetResponseCode"`
	ResponseLatency          int64  `json:"responseLatency"`
	UserIP                   string `json:"userIP"`
	UserAgentHeader          string `json:"userAgentHeader"`
	ResponseCacheHit         bool   `json:"responseCacheHit"`
	BackendLatency           int64  `json:"backendLatency"`
	EventType                string `json:"eventType"`
	APIResourceTemplate      string `json:"apiResourceTemplate"`
	ResponseMediationLatency int64  `json:"responseMediationLatency"`
	APIContext               string `json:"apiContext"`
}

// DefaultResponseEvent represents the default response event data.
type DefaultResponseEvent struct {
	RequestTimestamp         string                 `json:"requestTimestamp"`
	CorrelationID            string                 `json:"correlationId"`
	KeyType                  string                 `json:"keyType"`
	APIID                    string                 `json:"apiId"`
	APIType                  string                 `json:"apiType"`
	APIName                  string                 `json:"apiName"`
	APIVersion               string                 `json:"apiVersion"`
	APICreator               string                 `json:"apiCreator"`
	APIMethod                string                 `json:"apiMethod"`
	APIContext               string                 `json:"apiContext"`
	APIResourceTemplate      string                 `json:"apiResourceTemplate"`
	APICreatorTenantDomain   string                 `json:"apiCreatorTenantDomain"`
	Destination              string                 `json:"destination"`
	ApplicationID            string                 `json:"applicationId"`
	ApplicationName          string                 `json:"applicationName"`
	ApplicationOwner         string                 `json:"applicationOwner"`
	OrganizationID           string                 `json:"organizationId"`
	EnvironmentID            string                 `json:"environmentId"`
	RegionID                 string                 `json:"regionId"`
	GatewayType              string                 `json:"gatewayType"`
	UserAgentHeader          string                 `json:"userAgent"`
	UserName                 string                 `json:"userName"`
	ProxyResponseCode        int                    `json:"proxyResponseCode"`
	TargetResponseCode       int                    `json:"targetResponseCode"`
	ResponseCacheHit         bool                   `json:"responseCacheHit"`
	ResponseLatency          int64                  `json:"responseLatency"`
	BackendLatency           int64                  `json:"backendLatency"`
	RequestMediationLatency  int64                  `json:"requestMediationLatency"`
	ResponseMediationLatency int64                  `json:"responseMediationLatency"`
	UserIP                   string                 `json:"userIP"`
	EventType                string                 `json:"eventType"`
	Platform                 string                 `json:"platform"`
	Properties               map[string]interface{} `json:"properties"`
}
