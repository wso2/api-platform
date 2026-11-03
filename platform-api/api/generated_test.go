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

package api

import "testing"

// enumCase is one generated enum: whether a value is one of its constants.
type enumCase struct {
	name   string
	valid  func(string) bool
	values []string
}

func TestGeneratedEnumsAcceptOnlyTheirConstants(t *testing.T) {
	cases := []enumCase{
		{"MeterType", func(v string) bool { return MeterType(v).Valid() },
			[]string{string(MeterTypeRequestCount), string(MeterTypeTokens)}},
		{"MeterRequestType", func(v string) bool { return MeterRequestType(v).Valid() },
			[]string{string(MeterRequestTypeRequestCount), string(MeterRequestTypeTokens)}},
		{"PriceFilterOperator", func(v string) bool { return PriceFilterOperator(v).Valid() },
			[]string{string(PriceFilterOperatorEquals), string(PriceFilterOperatorNotEquals),
				string(PriceFilterOperatorIn), string(PriceFilterOperatorNotIn), string(PriceFilterOperatorStartsWith),
				string(PriceFilterOperatorGreaterThan), string(PriceFilterOperatorGreaterThanOrEqual),
				string(PriceFilterOperatorLessThan), string(PriceFilterOperatorLessThanOrEqual)}},
		{"SubscriptionBillingStatus", func(v string) bool { return SubscriptionBillingStatus(v).Valid() },
			[]string{string(SubscriptionBillingStatusActive), string(SubscriptionBillingStatusPastDue),
				string(SubscriptionBillingStatusUnpaid), string(SubscriptionBillingStatusCancelled)}},
		{"SubscriptionPlanPricePricingModel", func(v string) bool { return SubscriptionPlanPricePricingModel(v).Valid() },
			[]string{string(SubscriptionPlanPricePricingModelPerUnit), string(SubscriptionPlanPricePricingModelVolume),
				string(SubscriptionPlanPricePricingModelGraduated)}},
		{"SubscriptionPlanPricingBillingPeriod", func(v string) bool { return SubscriptionPlanPricingBillingPeriod(v).Valid() },
			[]string{string(SubscriptionPlanPricingBillingPeriodMonth), string(SubscriptionPlanPricingBillingPeriodYear)}},
		{"SubscriptionPlanPricingPricingType", func(v string) bool { return SubscriptionPlanPricingPricingType(v).Valid() },
			[]string{string(SubscriptionPlanPricingPricingTypeFlatRate), string(SubscriptionPlanPricingPricingTypeUsageBased)}},
		{"SubscriptionPlanPricingRequestBillingPeriod",
			func(v string) bool { return SubscriptionPlanPricingRequestBillingPeriod(v).Valid() },
			[]string{string(SubscriptionPlanPricingRequestBillingPeriodMonth),
				string(SubscriptionPlanPricingRequestBillingPeriodYear)}},
		{"SubscriptionPlanPricingRequestPricingType",
			func(v string) bool { return SubscriptionPlanPricingRequestPricingType(v).Valid() },
			[]string{string(SubscriptionPlanPricingRequestPricingTypeFlatRate),
				string(SubscriptionPlanPricingRequestPricingTypeUsageBased)}},
		{"ListSubscriptionPlansParamsStatus", func(v string) bool { return ListSubscriptionPlansParamsStatus(v).Valid() },
			[]string{string(ListSubscriptionPlansParamsStatusACTIVE), string(ListSubscriptionPlansParamsStatusINACTIVE)}},
		{"PublicationSelectionSourceStatus", func(v string) bool { return PublicationSelectionSourceStatus(v).Valid() },
			[]string{string(DELETED), string(UNCHANGED), string(UPDATED)}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, value := range c.values {
				if !c.valid(value) {
					t.Errorf("%s(%q) should be valid", c.name, value)
				}
			}
			for _, value := range []string{"", "UNKNOWN", "month"} {
				if c.valid(value) {
					t.Errorf("%s(%q) should not be valid", c.name, value)
				}
			}
		})
	}
}
