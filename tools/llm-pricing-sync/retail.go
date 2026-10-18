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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
)

const retailPricesURL = "https://prices.azure.com/api/retail/prices"

// retailPage is one page of the Retail Prices API.
type retailPage struct {
	Items        []meter `json:"Items"`
	NextPageLink string  `json:"NextPageLink"`
}

// meterRegions are the regions whose meters are read: the reference region
// for Global and Regional prices, and one region per Data Zone.
func meterRegions() []string {
	regions := make([]string, 0, len(dataZoneRegions))
	for region := range dataZoneRegions {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// fetchMeters reads every Foundry Models consumption meter of the given
// regions from baseURL, following the API's pagination.
func fetchMeters(ctx context.Context, client *http.Client, baseURL string, regions []string) ([]meter, error) {
	var all []meter
	for _, region := range regions {
		filter := fmt.Sprintf("serviceName eq 'Foundry Models' and armRegionName eq '%s' and type eq 'Consumption'", region)
		next := baseURL + "?api-version=2023-01-01-preview&$filter=" + url.QueryEscape(filter)
		for next != "" {
			page, err := fetchPage(ctx, client, next)
			if err != nil {
				return nil, fmt.Errorf("fetching %s meters: %w", region, err)
			}
			all = append(all, page.Items...)
			next = page.NextPageLink
		}
	}
	return all, nil
}

func fetchPage(ctx context.Context, client *http.Client, pageURL string) (retailPage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return retailPage{}, fmt.Errorf("building request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return retailPage{}, fmt.Errorf("calling the Retail Prices API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return retailPage{}, fmt.Errorf("the Retail Prices API returned %s", resp.Status)
	}
	var page retailPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return retailPage{}, fmt.Errorf("decoding a Retail Prices API page: %w", err)
	}
	return page, nil
}

// readMeters loads meters saved from the API, either a JSON array of items or
// a whole API page, so a sync can be reproduced offline.
func readMeters(paths []string) ([]meter, error) {
	var all []meter
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading meters %s: %w", path, err)
		}
		var items []meter
		if err := json.Unmarshal(data, &items); err != nil {
			var page retailPage
			if pageErr := json.Unmarshal(data, &page); pageErr != nil {
				return nil, fmt.Errorf("decoding meters %s: %w", path, err)
			}
			items = page.Items
		}
		all = append(all, items...)
	}
	return all, nil
}
