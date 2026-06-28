//go:build integration

/*
 *  Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 *  WSO2 LLC. licenses this file to you under the Apache License,
 *  Version 2.0 (the "License"); you may not use this file except
 *  in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing,
 *  software distributed under the License is distributed on an
 *  "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 *  KIND, either express or implied. See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

package integration

import (
	"fmt"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerWebBrokerSteps wires the WebBroker API repository scenario.
func registerWebBrokerSteps(ctx *godog.ScenarioContext, w *world) {
	ctx.Step(`^I create (\d+) WebBroker APIs in the project$`, w.createWebBrokerAPIs)
	ctx.Step(`^reading the first WebBroker API back returns lifecycle status "([^"]*)"$`, w.firstWebBrokerStatusIs)
	ctx.Step(`^paging WebBroker APIs (\d+) at a time covers all (\d+) without overlap$`, w.pageWebBrokerAPIs)
	ctx.Step(`^listing WebBroker APIs by project returns (\d+)$`, w.listWebBrokerByProject)
	ctx.Step(`^I update the first WebBroker API to status "([^"]*)"$`, w.updateFirstWebBrokerStatus)
	ctx.Step(`^I delete the first WebBroker API$`, w.deleteFirstWebBroker)
}

func (w *world) createWebBrokerAPIs(n int) error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	w.webbrokerHandles = w.webbrokerHandles[:0]
	for i := range n {
		handle := fmt.Sprintf("wb-api-%d-%s", i, id()[:6])
		api := &model.WebBrokerAPI{
			Handle:           handle,
			Name:             fmt.Sprintf("wb api %d", i),
			Version:          "v1.0",
			OrganizationUUID: w.orgID,
			ProjectUUID:      w.projID,
			LifeCycleStatus:  "CREATED",
			CreatedBy:        "it-user",
		}
		if err := repo.Create(api); err != nil {
			return fmt.Errorf("[%s] create WebBroker API %d failed: %w", w.it.driver, i, err)
		}
		if api.UUID == "" {
			return fmt.Errorf("[%s] create WebBroker API %d did not populate the generated UUID", w.it.driver, i)
		}
		w.webbrokerHandles = append(w.webbrokerHandles, handle)
	}
	return nil
}

func (w *world) firstWebBrokerStatusIs(status string) error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	got, err := repo.GetByHandle(w.webbrokerHandles[0], w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByHandle(%s) failed: %w", w.it.driver, w.webbrokerHandles[0], err)
	}
	if got == nil {
		return fmt.Errorf("[%s] GetByHandle(%s): want the WebBroker API, got nil", w.it.driver, w.webbrokerHandles[0])
	}
	if got.LifeCycleStatus != status || got.ProjectUUID != w.projID {
		return fmt.Errorf("[%s] WebBroker API round-trip mismatch: status=%q project=%q", w.it.driver, got.LifeCycleStatus, got.ProjectUUID)
	}
	return nil
}

func (w *world) pageWebBrokerAPIs(pageSize, total int) error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	seen := map[string]bool{}
	for offset := 0; offset < total; offset += pageSize {
		page, err := repo.List(w.orgID, "", pageSize, offset)
		if err != nil {
			return fmt.Errorf("[%s] List(%d,%d) failed: %w", w.it.driver, pageSize, offset, err)
		}
		want := pageSize
		if rem := total - offset; rem < want {
			want = rem
		}
		if len(page) != want {
			return fmt.Errorf("[%s] List offset %d: want %d, got %d", w.it.driver, offset, want, len(page))
		}
		for _, a := range page {
			if seen[a.UUID] {
				return fmt.Errorf("[%s] pagination overlap at offset %d: UUID %s seen twice", w.it.driver, offset, a.UUID)
			}
			seen[a.UUID] = true
		}
	}
	if len(seen) != total {
		return fmt.Errorf("[%s] paging covered %d rows, want %d", w.it.driver, len(seen), total)
	}
	return nil
}

func (w *world) listWebBrokerByProject(want int) error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	list, err := repo.List(w.orgID, w.projID, 100, 0)
	if err != nil {
		return fmt.Errorf("[%s] List by project failed: %w", w.it.driver, err)
	}
	if len(list) != want {
		return fmt.Errorf("[%s] List by project: want %d, got %d", w.it.driver, want, len(list))
	}
	count, err := repo.CountByProject(w.orgID, w.projID)
	if err != nil {
		return fmt.Errorf("[%s] CountByProject failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] CountByProject: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) updateFirstWebBrokerStatus(status string) error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	got, err := repo.GetByHandle(w.webbrokerHandles[0], w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] GetByHandle(%s) failed: %w", w.it.driver, w.webbrokerHandles[0], err)
	}
	got.LifeCycleStatus = status
	got.UpdatedBy = "it-user"
	if err := repo.Update(got); err != nil {
		return fmt.Errorf("[%s] Update WebBroker API failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) deleteFirstWebBroker() error {
	repo := repository.NewWebBrokerAPIRepo(w.it.db)
	if err := repo.Delete(w.webbrokerHandles[0], w.orgID); err != nil {
		return fmt.Errorf("[%s] Delete WebBroker API failed: %w", w.it.driver, err)
	}
	w.webbrokerHandles = w.webbrokerHandles[1:]
	return nil
}
