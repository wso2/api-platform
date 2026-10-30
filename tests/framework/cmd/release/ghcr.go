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
	"io"
	"net/http"
	"net/url"
)

// ghcrVersion is one package version as GitHub's list-package-versions API returns it.
type ghcrVersion struct {
	ID       int64 `json:"id"`
	Metadata struct {
		Container struct {
			Tags []string `json:"tags"`
		} `json:"container"`
	} `json:"metadata"`
}

// ghcrClient talks to the GitHub Packages (container) REST API for one organization's package.
type ghcrClient struct {
	baseURL    string
	httpClient *http.Client
	token      string
	org        string
	pkg        string
}

const (
	versionsPageSize = 100
	maxVersionPages  = 50
)

// listVersions returns every version of the package, each with the tags currently applied to it.
func (c *ghcrClient) listVersions(ctx context.Context) ([]ghcrVersion, error) {
	var all []ghcrVersion
	for page := 1; ; page++ {
		versions, err := c.listVersionsPage(ctx, page)
		if err != nil {
			return nil, err
		}
		if page > maxVersionPages && len(versions) > 0 {
			return nil, fmt.Errorf("listing package versions: more than %d pages of %d versions",
				maxVersionPages, versionsPageSize)
		}
		all = append(all, versions...)
		if len(versions) < versionsPageSize {
			return all, nil
		}
	}
}

func (c *ghcrClient) listVersionsPage(ctx context.Context, page int) ([]ghcrVersion, error) {
	reqURL := fmt.Sprintf("%s/orgs/%s/packages/container/%s/versions?per_page=%d&page=%d",
		c.baseURL, url.PathEscape(c.org), url.PathEscape(c.pkg), versionsPageSize, page)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("listing package versions: %s: %s", resp.Status, body)
	}
	var versions []ghcrVersion
	if err := json.Unmarshal(body, &versions); err != nil {
		return nil, fmt.Errorf("decoding package versions: %w", err)
	}
	return versions, nil
}

// deleteVersion deletes one package version by its ID.
func (c *ghcrClient) deleteVersion(ctx context.Context, id int64) error {
	reqURL := fmt.Sprintf("%s/orgs/%s/packages/container/%s/versions/%d",
		c.baseURL, url.PathEscape(c.org), url.PathEscape(c.pkg), id)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, reqURL, nil)
	if err != nil {
		return err
	}
	c.authorize(req)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting package version %d: %s: %s", id, resp.Status, body)
	}
	return nil
}

func (c *ghcrClient) authorize(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
}
