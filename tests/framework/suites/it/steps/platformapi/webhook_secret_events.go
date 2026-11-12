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

package platformapi

import (
	"context"
	"crypto/sha3"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/cucumber/godog"

	"github.com/wso2/api-platform/tests/framework/core/cleanup"
	"github.com/wso2/api-platform/tests/framework/core/util/httpx"
	"github.com/wso2/api-platform/tests/framework/core/util/tcontext"
	stepscommon "github.com/wso2/api-platform/tests/framework/suites/it/steps/common"
)

// webhookSecretHashLabel prefixes a webhook secret before it is hashed into the envelope's
// secret_hash. It is the label platform-api (utils.WebhookSecretHash) and the API Portal
// (envelopeCrypto.js) both use; the three implementations are pinned by shared test vectors.
const webhookSecretHashLabel = "api-portal-webhook-secret-hash-v1:"

// portalEventModifier matches one optional clause after the signing secret of a portal event.
var portalEventModifier = regexp.MustCompile(
	`naming the secret "([^"]*)"|naming the secret hash "([^"]*)"|without a secret hash|with a tampered body`)

// webhookSecretHash is the lowercase hex SHA3-256 of the labelled secret.
func webhookSecretHash(secret string) string {
	sum := sha3.Sum256([]byte(webhookSecretHashLabel + secret))
	return hex.EncodeToString(sum[:])
}

// RegisterWebhookSecretEvents binds the step that plays an API Portal whose webhook subscriber
// holds a secret platform-api issued: the event is signed with that secret and carries its hash,
// so platform-api finds the secret from the organization and the hash.
func RegisterWebhookSecretEvents(sc *godog.ScenarioContext, s *Steps) {
	sc.Step(`^the API Portal sends the "([^"]*)" event to the control plane signed with the secret "([^"]*)"((?: (?:naming the secret "[^"]*"|naming the secret hash "[^"]*"|without a secret hash|with a tampered body))*) with data:$`,
		s.sendSecretSignedPortalEvent)
	sc.Step(`^a webhook secret "([^"]*)" exists in the second organization and its value is stored as "([^"]*)"$`,
		s.createSecondOrganizationWebhookSecret)
	sc.Step(`^the hash of the secret "([^"]*)" should equal the stored "([^"]*)"$`, s.secretHashEqualsStored)
	sc.Step(`^I store the hash of the secret "([^"]*)" as "([^"]*)"$`, s.storeSecretHash)
}

// storedSecret resolves the plaintext a scenario stored under key.
func storedSecret(ctx context.Context, key string) (string, error) {
	secret, err := stepscommon.StoredValue(ctx, strings.TrimSpace(key))
	if err != nil {
		return "", fmt.Errorf("the secret stored as %q is unavailable: %w", key, err)
	}
	if secret == "" {
		return "", fmt.Errorf("the secret stored as %q is empty", key)
	}
	return secret, nil
}

// sendSecretSignedPortalEvent signs with the secret stored as secretKey. By default the envelope
// names that same secret by its hash; the modifiers name another secret or a literal hash, drop
// the hash, or change the body after it is signed.
func (s *Steps) sendSecretSignedPortalEvent(ctx context.Context, eventType, secretKey, modifiers string, data *godog.DocString) error {
	secret, err := storedSecret(ctx, secretKey)
	if err != nil {
		return err
	}
	spec := portalEventSpec{eventType: eventType, data: data.Content, secret: secret, secretHash: webhookSecretHash(secret)}
	for _, m := range portalEventModifier.FindAllStringSubmatch(modifiers, -1) {
		switch {
		case strings.HasPrefix(m[0], "naming the secret hash"):
			literal, err := stepscommon.Expand(ctx, m[2])
			if err != nil {
				return err
			}
			spec.secretHash = literal
		case strings.HasPrefix(m[0], "naming the secret"):
			named, err := storedSecret(ctx, m[1])
			if err != nil {
				return err
			}
			spec.secretHash = webhookSecretHash(named)
		case m[0] == "without a secret hash":
			spec.secretHash = ""
		case m[0] == "with a tampered body":
			spec.tamper = true
		}
	}
	return s.postPortalEvent(ctx, spec)
}

// storeSecretHash computes the hash a portal would carry for the secret stored as secretKey.
func (s *Steps) storeSecretHash(ctx context.Context, secretKey, key string) error {
	secret, err := storedSecret(ctx, secretKey)
	if err != nil {
		return err
	}
	return tcontext.Set(ctx, strings.TrimSpace(key), webhookSecretHash(secret))
}

// secretHashEqualsStored checks the hash of a stored secret against a stored value, such as the
// hash platform-api listed for the version.
func (s *Steps) secretHashEqualsStored(ctx context.Context, secretKey, key string) error {
	secret, err := storedSecret(ctx, secretKey)
	if err != nil {
		return err
	}
	want, err := stepscommon.StoredValue(ctx, strings.TrimSpace(key))
	if err != nil {
		return err
	}
	if got := webhookSecretHash(secret); got != want {
		return fmt.Errorf("the hash of the secret stored as %q is %q, but %q holds %q", secretKey, got, key, want)
	}
	return nil
}

// secondOrganizationWebhookSecretKind is removed as the second organization's administrator:
// the default administrator cleanup runs as cannot reach another organization's secrets.
var secondOrganizationWebhookSecretKind = cleanup.Kind{Name: "platform-api-second-org-webhook-secret", Order: 95}

// createSecondOrganizationWebhookSecret creates a webhook secret in the control plane's second
// organization and stores its value. It does not go through the request funnel, which refuses a
// create made as that organization because it could not delete the result; this step registers
// its own cleanup instead.
func (s *Steps) createSecondOrganizationWebhookSecret(ctx context.Context, handle, key string) error {
	handle, err := stepscommon.Expand(ctx, handle)
	if err != nil {
		return err
	}
	base, err := s.baseURL()
	if err != nil {
		return err
	}
	token, err := actorBearer(ctx, base, secondOrganizationAdmin)
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{"id": handle, "name": "Second organization secret"})
	if err != nil {
		return err
	}
	resp, err := s.client.Do(ctx, httpx.Request{
		Method: http.MethodPost, URL: base + apiBase + "/webhook-secrets", Body: body,
		Headers: map[string]string{"Authorization": "Bearer " + token, "Content-Type": "application/json"},
	}, 0, 0)
	if err != nil {
		return fmt.Errorf("creating a webhook secret in the second organization: %w", err)
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("creating a webhook secret in the second organization: %s", resp.Describe())
	}
	var created struct {
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(resp.Body, &created); err != nil || created.Secret == "" {
		return fmt.Errorf("the created webhook secret carries no value: %s", resp.Describe())
	}

	reg, err := cleanup.Of(ctx)
	if err == nil {
		err = reg.RegisterDeleter(secondOrganizationWebhookSecretKind, func(ctx context.Context, res cleanup.Resource) error {
			base, err := s.baseURL()
			if err != nil {
				return err
			}
			token, err := actorBearer(ctx, base, secondOrganizationAdmin)
			if err != nil {
				return err
			}
			deleted, err := s.client.Do(ctx, httpx.Request{
				Method: http.MethodDelete, URL: base + apiBase + "/webhook-secrets/" + res.ID,
				Headers: map[string]string{"Authorization": "Bearer " + token},
			}, 0, 0)
			if err != nil {
				return err
			}
			if deleted.StatusCode != http.StatusNotFound && !deleted.Succeeded() {
				return fmt.Errorf("deleting the second organization's webhook secret %q: %s", res.ID, deleted.Describe())
			}
			return nil
		})
	}
	if err == nil {
		err = reg.Register(cleanup.Resource{Kind: secondOrganizationWebhookSecretKind, ID: handle, Actor: "admin",
			Description: "created in the second organization"})
	}
	if err != nil {
		return fmt.Errorf("registering the second organization's webhook secret %q for cleanup: %w", handle, err)
	}
	return tcontext.Set(ctx, strings.TrimSpace(key), created.Secret)
}
