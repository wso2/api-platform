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
 *  KIND, either express or implied.  See the License for the
 *  specific language governing permissions and limitations
 *  under the License.
 */

// Package azureauthentication gets Microsoft Entra ID access tokens for the gateway's own Azure identity,
// for policies that call Azure services: AKS Workload Identity, a managed identity of the host
// (IMDS on a VM or AKS node), or a managed identity of the Azure Container Apps app the gateway
// runs in.
//
// Get returns one credential per identity for the whole gateway, so every policy that uses the
// same identity shares its token cache. Token gets a token for one scope:
//
//	cred, err := azureauthentication.Get(azureauthentication.Identity{Type: azureauthentication.ManagedIdentity})
//	if err != nil {
//		return err // the host can't provide the identity
//	}
//	token, err := cred.Token(ctx, "https://cognitiveservices.azure.com/.default", azureauthentication.DefaultTokenTimeout)
//
// A failed request is a *TokenError. Its Reason is safe to return to a client or a fault policy;
// its Error text, for logs only, adds the SDK's error with any token withheld.
package azureauthentication

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// The authentication types: how the gateway gets its token.
const (
	// WorkloadIdentity exchanges the pod's AKS Workload Identity federated token for an access
	// token from Microsoft Entra ID.
	WorkloadIdentity = "workload-identity"

	// ManagedIdentity gets the access token for an identity assigned to the host: from the Azure
	// Instance Metadata Service (IMDS) on a VM, VM scale set or AKS node, or from the host's own
	// managed identity endpoint on App Service or Container Apps.
	ManagedIdentity = "managed-identity"

	// ContainerApps gets the access token for a managed identity of the Azure Container Apps app the
	// gateway runs in, from the identity endpoint Container Apps sets in IDENTITY_ENDPOINT.
	ContainerApps = "container-apps"
)

// DefaultTokenTimeout bounds one token fetch, retries included. The retry settings are tuned to
// fit it.
const DefaultTokenTimeout = 10 * time.Second

var (
	// scopePattern accepts a single client-credentials scope: a resource followed by /.default.
	scopePattern = regexp.MustCompile(`^\S+/\.default$`)

	// guidPattern matches an Entra application (client) ID.
	guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

	// tenantPattern matches a tenant GUID or a domain name. The tenant is placed in the token URL
	// path, so anything that could change the path ("/", "?", "%", "..") is rejected.
	tenantPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*(\.[A-Za-z0-9-]+)*$`)
)

// Types returns the authentication types in the order the docs list them.
func Types() []string {
	names := make([]string, len(authenticationKinds))
	for i, k := range authenticationKinds {
		names[i] = k.name
	}
	return names
}

// Identity names a credential. An empty ClientID means AZURE_CLIENT_ID for workload identity and the
// host's default identity otherwise. An empty TenantID means AZURE_TENANT_ID; only workload identity
// uses a tenant.
type Identity struct {
	Type     string
	ClientID string
	TenantID string
}

// Validate checks Type, and that ClientID is a GUID and TenantID a tenant ID or domain name when
// set. Only workload identity takes a TenantID.
func (id Identity) Validate() error {
	kind := kindOf(id.Type)
	switch {
	case id.Type == "":
		return fmt.Errorf("an authentication type is required: %s", typeNames())
	case kind == nil:
		return fmt.Errorf("authentication type %q is not supported: use %s", id.Type, typeNames())
	case id.ClientID != "" && !guidPattern.MatchString(id.ClientID):
		return errors.New("the client ID must be an application (client) ID in GUID form")
	case id.TenantID != "" && !kind.usesTenant:
		return fmt.Errorf("a tenant ID is not used with authentication type %q", id.Type)
	case id.TenantID != "" && !tenantPattern.MatchString(id.TenantID):
		return errors.New("the tenant ID must be a tenant ID (GUID) or a domain name")
	}
	return nil
}

// ValidateScope checks that scope is one resource's /.default scope, such as
// https://cognitiveservices.azure.com/.default.
func ValidateScope(scope string) error {
	if !scopePattern.MatchString(scope) {
		return errors.New("the scope must be a single scope ending in /.default, such as https://cognitiveservices.azure.com/.default")
	}
	return nil
}

// typeNames lists the authentication types, quoted, for messages.
func typeNames() string {
	names := Types()
	for i, n := range names {
		names[i] = strconv.Quote(n)
	}
	return listing(names, "or")
}

// listing joins items for a message: "a", "a or b", "a, b or c".
func listing(items []string, conjunction string) string {
	if len(items) == 1 {
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " " + conjunction + " " + items[len(items)-1]
}
