/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package utils

import "github.com/spf13/cobra"

const (
	FlagName                   = "display-name"
	FlagType                   = "type"
	FlagContext                = "context"
	FlagServer                 = "server"
	FlagAdminServer            = "admin-server"
	FlagAuth                   = "auth"
	FlagUsername               = "username"
	FlagPassword               = "password"
	FlagToken                  = "token"
	FlagAPIKey                 = "api-key"
	FlagPlatform               = "platform"
	FlagNoInteractive          = "no-interactive"
	FlagPasswordEnv            = "password-env"
	FlagOutput                 = "output"
	FlagFile                   = "file"
	FlagFormat                 = "format"
	FlagVersion                = "version"
	FlagID                     = "id"
	FlagLimit                  = "limit"
	FlagOffset                 = "offset"
	FlagAPIID                  = "api-id"
	FlagSubID                  = "sub-id"
	FlagConfirm                = "confirm"
	FlagDockerRegistry         = "docker-registry"
	FlagImageTag               = "image-tag"
	FlagGatewayBuilder         = "gateway-builder"
	FlagGatewayControllerImage = "gateway-controller-base-image"
	FlagRouterBaseImage        = "router-base-image"
	FlagHeader                 = "header"
	FlagPolicyId               = "policy-id"
	FlagOrgID                  = "org"
	FlagRequestCount           = "request-count"
	FlagEventCount             = "event-count"
	FlagPricingModel           = "pricing-model"
	FlagBillingPeriod          = "billing-period"
	FlagFlatAmount             = "flat-amount"
	FlagUnitAmount             = "unit-amount"
	FlagCurrency               = "currency"
	FlagInsecure               = "insecure"
	FlagStatus                 = "status"
	FlagApplicationID          = "application-id"
	FlagSubscriptionPlan       = "subscription-plan"
	FlagAPIKeyID               = "api-key-id"
	FlagKeyName                = "key-name"
	FlagExpiresAt              = "expires-at"
	FlagPropertyName           = "name"
	FlagAppID                  = "app-id"
	FlagDescription            = "description"
	FlagGateway                = "gateway"
	FlagPlanName               = "plan-name"
	FlagBillingPlan            = "billing-plan"
	FlagStopOnQuotaReach       = "stop-on-quota-reach"
	FlagThrottleLimitCount     = "throttle-limit-count"
	FlagThrottleLimitUnit      = "throttle-limit-unit"
	FlagExpiryTime             = "expiry-time"
	FlagSubscriptionPlanID     = "subscription-plan-id"
	FlagSubscriptionToken      = "subscription-token"
	FlagBillingCustomerID      = "billing-customer-id"
	FlagBillingSubscriptionID  = "billing-subscription-id"
	FlagReferenceID            = "reference-id"
	FlagGatewayType            = "gateway-type"
	FlagUseSpec                = "use-spec"
	FlagProjectID              = "project-id"
)

var shortFlags = map[string]string{
	FlagName:          "n",
	FlagServer:        "s",
	FlagNoInteractive: "y",
	FlagOutput:        "o",
	FlagFile:          "f",
	FlagVersion:       "v",
}

func GetShortFlags() []string {
	values := make([]string, 0, len(shortFlags))
	for _, v := range shortFlags {
		values = append(values, v)
	}
	return values
}

func AddStringFlag(cmd *cobra.Command, flagName string, p *string, defaultValue, usage string) {
	if short, hasShort := shortFlags[flagName]; hasShort {
		cmd.Flags().StringVarP(p, flagName, short, defaultValue, usage)
	} else {
		cmd.Flags().StringVar(p, flagName, defaultValue, usage)
	}
}

func AddBoolFlag(cmd *cobra.Command, flagName string, p *bool, defaultValue bool, usage string) {
	if short, hasShort := shortFlags[flagName]; hasShort {
		cmd.Flags().BoolVarP(p, flagName, short, defaultValue, usage)
	} else {
		cmd.Flags().BoolVar(p, flagName, defaultValue, usage)
	}
}

func AddIntFlag(cmd *cobra.Command, flagName string, p *int, defaultValue int, usage string) {
	if short, hasShort := shortFlags[flagName]; hasShort {
		cmd.Flags().IntVarP(p, flagName, short, defaultValue, usage)
	} else {
		cmd.Flags().IntVar(p, flagName, defaultValue, usage)
	}
}
