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

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// meter is one item of the Azure Retail Prices API.
type meter struct {
	ProductName   string  `json:"productName"`
	SkuName       string  `json:"skuName"`
	ArmRegionName string  `json:"armRegionName"`
	UnitPrice     float64 `json:"unitPrice"`
	UnitOfMeasure string  `json:"unitOfMeasure"`
	Type          string  `json:"type"`
}

// referenceRegion supplies Global prices, which are the same everywhere, and
// Regional prices, which are not: other regions override the pricing file.
const referenceRegion = "eastus2"

// dataZoneRegions names the region whose Data Zone meters price each zone.
var dataZoneRegions = map[string]string{
	"eastus2":       "us",
	"swedencentral": "eu",
	"japaneast":     "apac",
}

// prices maps a pricing-file key, such as "azure_ai/us/deepseek-v3.2", to the
// rate fields the meters set for it.
type prices map[string]map[string]float64

// buildPrices turns meters into pricing entries. Entries the meters do not
// give both an input and an output rate are dropped and named in skipped, so
// a partial meter set never replaces a complete entry.
func buildPrices(meters []meter) (prices, []string, error) {
	out := prices{}
	for _, m := range meters {
		key, field, rate, ok, err := priceMeter(m)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		if err := setRate(out, key, field, rate, m.SkuName); err != nil {
			return nil, nil, err
		}
	}

	var skipped []string
	for key, fields := range out {
		if _, hasIn := fields[fieldInput]; !hasIn {
			skipped = append(skipped, key)
		} else if _, hasOut := fields[fieldOutput]; !hasOut {
			skipped = append(skipped, key)
		}
	}
	sort.Strings(skipped)
	for _, key := range skipped {
		delete(out, key)
	}
	return out, skipped, nil
}

// priceMeter resolves one meter to the keys and field it prices. ok is false
// for meters this tool does not track.
func priceMeter(m meter) (keys []string, field string, rate float64, ok bool, err error) {
	if m.Type != "Consumption" {
		return nil, "", 0, false, nil
	}
	rate, ok = perTokenRate(m)
	if !ok {
		return nil, "", 0, false, nil
	}
	spec, class, ok, err := matchMeter(m)
	if err != nil || !ok {
		return nil, "", 0, false, err
	}
	if class.longContext && !longContextAt272k[spec.keys[0]] {
		return nil, "", 0, false, nil
	}
	prefix, ok := entryPrefix(class.deployment, m.ArmRegionName)
	if !ok {
		return nil, "", 0, false, nil
	}
	for _, k := range spec.keys {
		keys = append(keys, string(spec.catalog)+"/"+prefix+k)
	}
	return keys, fieldName(class), rate, true, nil
}

// setRate records one rate under every key the meter prices. A second meter
// giving the same field a different rate is an error, not a silent pick.
func setRate(out prices, keys []string, field string, rate float64, sku string) error {
	for _, key := range keys {
		fields, ok := out[key]
		if !ok {
			fields = map[string]float64{}
			out[key] = fields
		}
		if prev, seen := fields[field]; seen && prev != rate {
			return fmt.Errorf("conflicting %s for %s: %v and %v (SKU %q)", field, key, prev, rate, sku)
		}
		fields[field] = rate
	}
	return nil
}

// matchMeter finds the model a meter belongs to. A SKU matches a stem when the
// tokens after it classify cleanly; matching two models is an error.
func matchMeter(m meter) (*modelSpec, meterClass, bool, error) {
	var (
		found *modelSpec
		class meterClass
	)
	sku := strings.ToLower(strings.TrimSpace(m.SkuName))
	for i := range models {
		spec := &models[i]
		if spec.product != m.ProductName {
			continue
		}
		for _, stem := range spec.stems {
			c, ok := classifyAfterStem(sku, strings.ToLower(stem))
			if !ok {
				continue
			}
			if found != nil && found != spec {
				return nil, meterClass{}, false, fmt.Errorf("SKU %q matches both %s and %s", m.SkuName, found.keys[0], spec.keys[0])
			}
			found, class = spec, c
		}
	}
	return found, class, found != nil, nil
}

// classifyAfterStem classifies what follows stem in sku, provided the stem is
// a whole-word prefix.
func classifyAfterStem(sku, stem string) (meterClass, bool) {
	if !strings.HasPrefix(sku, stem) || len(sku) == len(stem) {
		return meterClass{}, false
	}
	if next := sku[len(stem)]; next != ' ' && next != '-' {
		return meterClass{}, false
	}
	return classify(splitTokens(sku[len(stem):]))
}

// perTokenRate converts a per-1K or per-1M price to a per-token rate, rounded
// to 12 significant digits so float division leaves no trailing noise.
func perTokenRate(m meter) (float64, bool) {
	var divisor float64
	switch m.UnitOfMeasure {
	case "1K":
		divisor = 1e3
	case "1M":
		divisor = 1e6
	default:
		return 0, false
	}
	rounded, err := strconv.ParseFloat(strconv.FormatFloat(m.UnitPrice/divisor, 'g', 12, 64), 64)
	if err != nil || rounded <= 0 {
		return 0, false
	}
	return rounded, true
}

// entryPrefix is the key segment for a deployment type, as azure-llm-cost
// looks it up. Global and Regional prices come from the reference region only.
func entryPrefix(d deployment, region string) (string, bool) {
	switch d {
	case deploymentGlobal:
		return "", region == referenceRegion
	case deploymentRegional:
		return "regional/", region == referenceRegion
	case deploymentDataZone:
		zone, ok := dataZoneRegions[region]
		return zone + "/", ok
	}
	return "", false
}
