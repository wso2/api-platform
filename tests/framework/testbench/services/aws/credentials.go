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

package aws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	// issuedKeyPrefix marks credentials issued by the STS service. The rest of the 20-character
	// access key ID is the zero-padded Unix expiry time, from which the secret and session token
	// are derived.
	issuedKeyPrefix = "ASIASTS"

	// issuedCredentialLifetime is how long STS-issued credentials remain valid.
	issuedCredentialLifetime = 15 * time.Second

	// expiryGrace accepts a request signed just before its credentials expired but received just
	// after, as AWS does for requests in flight.
	expiryGrace = 5 * time.Second

	// issuerKey derives STS-issued secrets and session tokens. It is a fixture, not a secret.
	issuerKey = "testbench-aws-sts-issuer-key"
)

// credential is the secret material and validity of one access key ID.
type credential struct {
	secretAccessKey string
	// sessionToken, when set, must accompany every request signed with this credential.
	sessionToken string
	// expiresAt is zero for a credential that never expires.
	expiresAt time.Time
	// permittedResources, when set, are the only resource paths below /<service>/<region> the
	// credential may invoke, each with everything beneath it. A nil list permits every resource.
	permittedResources []string
}

// fixtureCredentials are the documented credentials features configure on the policy.
var fixtureCredentials = map[string]credential{
	"AKIATESTPERMANENT001": {secretAccessKey: "testPermanentSecretAccessKey000000000001"},
	"AKIATESTPERMANENT002": {secretAccessKey: "testPermanentSecretAccessKey000000000002"},
	"AKIATESTWORKLOAD0001": {secretAccessKey: "testWorkloadSecretAccessKey0000000000001"},
	"AKIATESTRESTRICTED01": {
		secretAccessKey:    "testRestrictedSecretAccessKey00000000001",
		permittedResources: []string{"/items"},
	},
	"ASIATESTTEMPORARY001": {
		secretAccessKey: "testTemporarySecretAccessKey000000000001",
		sessionToken:    "testTemporarySessionToken0001",
	},
	"ASIATESTEXPIRED00001": {
		secretAccessKey: "testExpiredSecretAccessKey00000000000001",
		sessionToken:    "testExpiredSessionToken0001",
		expiresAt:       time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC),
	},
}

// lookupCredential returns the credential for an access key ID, fixture or STS-issued.
func lookupCredential(accessKeyID string) (credential, bool) {
	if c, ok := fixtureCredentials[accessKeyID]; ok {
		return c, true
	}
	return issuedCredential(accessKeyID)
}

// issueCredentials returns the access key ID and credential STS issues at now, valid for
// lifetime.
func issueCredentials(now time.Time, lifetime time.Duration) (string, credential) {
	expiry := now.Add(lifetime).Truncate(time.Second)
	accessKeyID := fmt.Sprintf("%s%013d", issuedKeyPrefix, expiry.Unix())
	c, _ := issuedCredential(accessKeyID)
	return accessKeyID, c
}

// issuedCredential re-derives an STS-issued credential from its access key ID.
func issuedCredential(accessKeyID string) (credential, bool) {
	digits, ok := strings.CutPrefix(accessKeyID, issuedKeyPrefix)
	if !ok || len(accessKeyID) != 20 {
		return credential{}, false
	}
	seconds, err := strconv.ParseInt(digits, 10, 64)
	if err != nil || seconds <= 0 {
		return credential{}, false
	}
	secret := derive("secret", accessKeyID)
	return credential{
		secretAccessKey: hex.EncodeToString(secret)[:40],
		sessionToken:    base64.RawURLEncoding.EncodeToString(derive("token", accessKeyID)),
		expiresAt:       time.Unix(seconds, 0).UTC(),
	}, true
}

func derive(purpose, accessKeyID string) []byte {
	mac := hmac.New(sha256.New, []byte(issuerKey))
	mac.Write([]byte(purpose + "\x00" + accessKeyID))
	return mac.Sum(nil)
}

// permits reports whether the credential may invoke resource, a path below /<service>/<region>.
func (c credential) permits(resource string) bool {
	if c.permittedResources == nil {
		return true
	}
	for _, permitted := range c.permittedResources {
		if underPath(resource, permitted) {
			return true
		}
	}
	return false
}

// underPath reports whether path is base or a path beneath it.
func underPath(path, base string) bool {
	return path == base || strings.HasPrefix(path, base+"/")
}

// expired reports whether the credential can no longer authenticate a request received at now.
func (c credential) expired(now time.Time) bool {
	return !c.expiresAt.IsZero() && now.After(c.expiresAt.Add(expiryGrace))
}
