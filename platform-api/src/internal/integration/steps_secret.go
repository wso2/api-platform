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
	"bytes"
	"fmt"

	"github.com/cucumber/godog"

	"platform-api/src/internal/model"
	"platform-api/src/internal/repository"
)

// registerSecretSteps wires the secret repository scenario, which verifies the
// encrypted-ciphertext round-trip, the dialect-aware paginated list, rotation
// and the reference-checked soft-delete on every engine.
func registerSecretSteps(ctx *godog.ScenarioContext, w *world) {
	ctx.Step(`^I create (\d+) secrets$`, w.createSecrets)
	ctx.Step(`^reading the first secret back returns its ciphertext and hash$`, w.firstSecretCipherRoundTrip)
	ctx.Step(`^the first secret reports type "([^"]*)" provider "([^"]*)" status "([^"]*)"$`, w.firstSecretMetadata)
	ctx.Step(`^checking existence of the first secret returns true$`, w.firstSecretExists)
	ctx.Step(`^checking existence of a missing secret returns false$`, w.missingSecretAbsent)
	ctx.Step(`^counting secrets returns (\d+)$`, w.countSecrets)
	ctx.Step(`^paging secrets (\d+) at a time covers all (\d+) without overlap$`, w.pageSecrets)
	ctx.Step(`^I rotate the first secret's ciphertext$`, w.rotateFirstSecret)
	ctx.Step(`^reading the first secret back returns the rotated ciphertext$`, w.firstSecretRotated)
	ctx.Step(`^I soft-delete the first secret$`, w.softDeleteFirstSecret)
	ctx.Step(`^reading the first secret back reports status "([^"]*)"$`, w.firstSecretStatusIs)
}

func (w *world) createSecrets(n int) error {
	repo := repository.NewSecretRepo(w.it.db)
	w.secretHandles = w.secretHandles[:0]
	for i := range n {
		handle := fmt.Sprintf("secret-%d-%s", i, id()[:6])
		cipher := []byte(fmt.Sprintf("cipher-%d-%s", i, id()))
		hash := "hash-" + id()
		secret := &model.Secret{
			OrganizationID: w.orgID,
			Handle:         handle,
			DisplayName:    fmt.Sprintf("Secret %d", i),
			Description:    "created by integration test",
			Ciphertext:     cipher,
			Hash:           hash,
			CreatedBy:      "it-user",
			// A scope row exercises the secret_scopes write (and its FK).
			Scopes: []model.SecretScope{{Scope: model.SecretScopeTypeOrg, ScopeValue: w.orgID}},
		}
		if err := repo.Create(secret); err != nil {
			return fmt.Errorf("[%s] create secret %d failed: %w", w.it.driver, i, err)
		}
		if i == 0 {
			w.firstSecretCipher = cipher
			w.firstSecretHash = hash
		}
		w.secretHandles = append(w.secretHandles, handle)
	}
	return nil
}

func (w *world) getFirstSecret() (*model.Secret, error) {
	repo := repository.NewSecretRepo(w.it.db)
	got, err := repo.GetByHandle(w.orgID, w.secretHandles[0])
	if err != nil {
		return nil, fmt.Errorf("[%s] GetByHandle(%s) failed: %w", w.it.driver, w.secretHandles[0], err)
	}
	return got, nil
}

func (w *world) firstSecretCipherRoundTrip() error {
	got, err := w.getFirstSecret()
	if err != nil {
		return err
	}
	if !bytes.Equal(got.Ciphertext, w.firstSecretCipher) {
		return fmt.Errorf("[%s] secret ciphertext did not round-trip: want %q, got %q", w.it.driver, w.firstSecretCipher, got.Ciphertext)
	}
	if got.Hash != w.firstSecretHash {
		return fmt.Errorf("[%s] secret hash did not round-trip: want %q, got %q", w.it.driver, w.firstSecretHash, got.Hash)
	}
	return nil
}

func (w *world) firstSecretMetadata(secretType, provider, status string) error {
	got, err := w.getFirstSecret()
	if err != nil {
		return err
	}
	if got.Type != secretType || got.Provider != provider || got.Status != status {
		return fmt.Errorf("[%s] secret metadata mismatch: type=%q provider=%q status=%q",
			w.it.driver, got.Type, got.Provider, got.Status)
	}
	return nil
}

func (w *world) firstSecretExists() error {
	repo := repository.NewSecretRepo(w.it.db)
	exists, err := repo.Exists(w.orgID, w.secretHandles[0])
	if err != nil {
		return fmt.Errorf("[%s] Exists failed: %w", w.it.driver, err)
	}
	if !exists {
		return fmt.Errorf("[%s] Exists: want true for %q", w.it.driver, w.secretHandles[0])
	}
	return nil
}

func (w *world) missingSecretAbsent() error {
	repo := repository.NewSecretRepo(w.it.db)
	exists, err := repo.Exists(w.orgID, "no-such-"+id())
	if err != nil {
		return fmt.Errorf("[%s] Exists(missing) failed: %w", w.it.driver, err)
	}
	if exists {
		return fmt.Errorf("[%s] Exists: want false for a random handle", w.it.driver)
	}
	return nil
}

func (w *world) countSecrets(want int) error {
	repo := repository.NewSecretRepo(w.it.db)
	count, err := repo.Count(w.orgID)
	if err != nil {
		return fmt.Errorf("[%s] Count failed: %w", w.it.driver, err)
	}
	if count != want {
		return fmt.Errorf("[%s] Count: want %d, got %d", w.it.driver, want, count)
	}
	return nil
}

func (w *world) pageSecrets(pageSize, total int) error {
	repo := repository.NewSecretRepo(w.it.db)
	seen := map[string]bool{}
	for offset := 0; offset < total; offset += pageSize {
		page, err := repo.List(w.orgID, pageSize, offset, nil)
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
		for _, s := range page {
			if seen[s.UUID] {
				return fmt.Errorf("[%s] pagination overlap at offset %d: UUID %s seen twice", w.it.driver, offset, s.UUID)
			}
			seen[s.UUID] = true
		}
	}
	if len(seen) != total {
		return fmt.Errorf("[%s] paging covered %d rows, want %d", w.it.driver, len(seen), total)
	}
	return nil
}

func (w *world) rotateFirstSecret() error {
	repo := repository.NewSecretRepo(w.it.db)
	got, err := w.getFirstSecret()
	if err != nil {
		return err
	}
	w.rotatedCipher = []byte("rotated-" + id())
	w.rotatedHash = "rotated-hash-" + id()
	got.Ciphertext = w.rotatedCipher
	got.Hash = w.rotatedHash
	got.UpdatedBy = "it-user"
	if err := repo.Update(got); err != nil {
		return fmt.Errorf("[%s] Update (rotate) failed: %w", w.it.driver, err)
	}
	return nil
}

func (w *world) firstSecretRotated() error {
	got, err := w.getFirstSecret()
	if err != nil {
		return err
	}
	if !bytes.Equal(got.Ciphertext, w.rotatedCipher) {
		return fmt.Errorf("[%s] rotated ciphertext did not persist: want %q, got %q", w.it.driver, w.rotatedCipher, got.Ciphertext)
	}
	if got.Hash != w.rotatedHash {
		return fmt.Errorf("[%s] rotated hash did not persist: want %q, got %q", w.it.driver, w.rotatedHash, got.Hash)
	}
	return nil
}

func (w *world) softDeleteFirstSecret() error {
	repo := repository.NewSecretRepo(w.it.db)
	// No artifact references the secret, so soft-delete deprecates it and returns
	// no references.
	refs, err := repo.FindRefsAndSoftDelete(w.orgID, w.secretHandles[0], "it-user")
	if err != nil {
		return fmt.Errorf("[%s] FindRefsAndSoftDelete failed: %w", w.it.driver, err)
	}
	if len(refs) != 0 {
		return fmt.Errorf("[%s] FindRefsAndSoftDelete: want no references, got %d", w.it.driver, len(refs))
	}
	return nil
}

func (w *world) firstSecretStatusIs(status string) error {
	got, err := w.getFirstSecret()
	if err != nil {
		return err
	}
	if got.Status != status {
		return fmt.Errorf("[%s] secret status: want %q, got %q", w.it.driver, status, got.Status)
	}
	return nil
}
