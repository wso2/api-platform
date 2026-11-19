/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cloudbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/retry"
)

// ErrCollectionNotServed marks a collection whose list route the platform answers with 404.
var ErrCollectionNotServed = errors.New("the platform does not serve this collection")

// maxListPages bounds how many pages one collection is read through.
const maxListPages = 100

// ListCollection reads every page of a collection.
func ListCollection(ctx context.Context, client *httpx.Client, base, token, collection string) ([]map[string]any, error) {
	return ListCollectionQuery(ctx, client, base, token, collection, nil)
}

// ListCollectionQuery reads every page of a collection with extra query parameters. The first
// request carries no paging parameters; when the platform reports that more items exist, the next
// pages are read with the limit and offset it advertises. A failure part way returns the items of
// the pages already read together with the error.
func ListCollectionQuery(ctx context.Context, client *httpx.Client, base, token, collection string, query url.Values) ([]map[string]any, error) {
	var all []map[string]any
	offset, limit := 0, 0
	for page := 0; page < maxListPages; page++ {
		params := url.Values{}
		for key, values := range query {
			params[key] = values
		}
		if page > 0 {
			params.Set("limit", strconv.Itoa(limit))
			params.Set("offset", strconv.Itoa(offset))
		}
		target := CloudAPIURL(base, collection)
		if len(params) > 0 {
			target += "?" + params.Encode()
		}
		resp, err := getListPage(ctx, client, target, token)
		if err != nil {
			return all, err
		}
		if resp.StatusCode == http.StatusNotFound && page == 0 {
			return nil, ErrCollectionNotServed
		}
		if resp.StatusCode != http.StatusOK {
			return all, ResponseError(resp)
		}
		items, err := ListItems(resp)
		if err != nil {
			return all, err
		}
		paging := listPaging(resp)
		if page > 0 && paging != nil && paging.Offset != nil && *paging.Offset != offset {
			// A page from the wrong offset can repeat items already read, so it is not kept.
			return all, fmt.Errorf("%s did not honour offset %d, it answered from offset %d", collection, offset, *paging.Offset)
		}
		all = append(all, items...)
		if paging == nil || len(items) == 0 {
			return all, nil
		}
		offset += len(items)
		if paging.Total == nil || offset >= *paging.Total {
			return all, nil
		}
		limit = max(len(items), 1)
		if paging.Limit != nil && *paging.Limit > 0 {
			limit = *paging.Limit
		}
	}
	return all, fmt.Errorf("%s holds more than %d pages; the rest were not read", collection, maxListPages)
}

// getListPage reads one page of a list. A transport error, a 5xx or a 429 is retried, since a
// listing that is missed leaves the run's resources in the organisation for good. Any other answer
// is returned for the caller to judge.
func getListPage(ctx context.Context, client *httpx.Client, target, token string) (*httpx.Response, error) {
	return retry.Until(ctx, retry.Options{Fast: true},
		func(ctx context.Context) (*httpx.Response, error) {
			resp, err := client.Do(ctx, httpx.Request{
				Method:  http.MethodGet,
				URL:     target,
				Headers: map[string]string{"Authorization": "Bearer " + token},
			}, 0, 0)
			if err != nil {
				return nil, retry.Transient(err)
			}
			if resp.StatusCode >= http.StatusInternalServerError || resp.StatusCode == http.StatusTooManyRequests {
				return nil, retry.Transient(ResponseError(resp))
			}
			return resp, nil
		},
		func(resp *httpx.Response) bool { return resp != nil })
}

// pageInfo is the paging block of a list response. A field the platform leaves out is nil.
type pageInfo struct {
	Total  *int `json:"total"`
	Offset *int `json:"offset"`
	Limit  *int `json:"limit"`
}

// listPaging returns the paging block of a list response, or nil when the response has none.
func listPaging(resp *httpx.Response) *pageInfo {
	var document struct {
		Pagination *pageInfo `json:"pagination"`
	}
	if err := json.Unmarshal(resp.Body, &document); err != nil {
		return nil
	}
	return document.Pagination
}
