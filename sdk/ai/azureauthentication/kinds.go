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

package azureauthentication

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azpolicy "github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

var (
	// Retries that fit DefaultTokenTimeout. Managed identity retries the codes the SDK lists for
	// IMDS (404, 410, 429 and 5xx); Entra ID retries azcore's defaults.
	entraRetry   = azpolicy.RetryOptions{MaxRetries: 2, RetryDelay: 200 * time.Millisecond, MaxRetryDelay: 2 * time.Second}
	managedRetry = azpolicy.RetryOptions{MaxRetries: 4, RetryDelay: time.Second, MaxRetryDelay: 4 * time.Second}

	// containerAppsRetry is Entra ID's: the Container Apps identity endpoint runs beside the app
	// and gets its tokens from Entra ID, so it fails the way Entra ID does, not the way IMDS does
	// while it starts.
	containerAppsRetry = azpolicy.RetryOptions{MaxRetries: 2, RetryDelay: 200 * time.Millisecond, MaxRetryDelay: 2 * time.Second}

	// transport is the credentials' HTTP transport. nil means the SDK's default client; tests
	// replace it.
	transport azpolicy.Transporter
)

// authenticationKind describes an authentication type: how its credential is built and how its
// token service behaves.
type authenticationKind struct {
	name    string // the Identity.Type value
	service string // the token service, as reasons and Service name it

	// unavailable begins Get's error when the credential can't be built.
	unavailable string

	// usesTenant: the identity is named by a tenant and a client ID, which default to
	// AZURE_TENANT_ID and AZURE_CLIENT_ID. Otherwise the identity is the host's, named by its client
	// ID or the host's default.
	usesTenant bool

	// oneFetchAtATime: the SDK fetches one token at a time per credential, behind a lock that
	// doesn't watch the caller's context, so all of a credential's scopes share one admission slot.
	oneFetchAtATime bool

	// imds: the token service may be IMDS, which answers 404 and 410 while it starts or updates,
	// and whose refusal of the host's default identity the SDK passes on without the response.
	imds bool

	newCredential func(Identity) (azcore.TokenCredential, error)
}

// authenticationKinds are the supported authentication types, in the order the docs list them.
var authenticationKinds = []*authenticationKind{
	{
		name:    WorkloadIdentity,
		service: "Microsoft Entra ID",
		unavailable: "workload identity is not available (the AKS Workload Identity webhook normally sets " +
			"AZURE_CLIENT_ID, AZURE_TENANT_ID and AZURE_FEDERATED_TOKEN_FILE)",
		usesTenant:      true,
		oneFetchAtATime: true,
		newCredential:   newWorkloadIdentityCredential,
	},
	{
		// A managed identity token comes from IMDS on a VM or AKS node, or from the host's own
		// endpoint on App Service, Container Apps or Arc.
		name:          ManagedIdentity,
		service:       "the managed identity endpoint",
		unavailable:   "managed identity is not available",
		imds:          true,
		newCredential: newManagedIdentityCredential,
	},
	{
		name:    ContainerApps,
		service: "the Container Apps identity endpoint",
		unavailable: "Container Apps managed identity is not available (Container Apps sets " +
			"IDENTITY_ENDPOINT and IDENTITY_HEADER when the container app has a managed identity)",
		newCredential: newContainerAppsCredential,
	},
}

// kindOf returns the authentication type called name, or nil if there is none.
func kindOf(name string) *authenticationKind {
	for _, k := range authenticationKinds {
		if k.name == name {
			return k
		}
	}
	return nil
}

// newWorkloadIdentityCredential builds the SDK credential for AKS Workload Identity. It reads
// AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_FEDERATED_TOKEN_FILE and AZURE_AUTHORITY_HOST, and the
// identity's ClientID and TenantID take precedence over the first two.
func newWorkloadIdentityCredential(id Identity) (azcore.TokenCredential, error) {
	return azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		ClientOptions: azcore.ClientOptions{Retry: entraRetry, Transport: transport},
		ClientID:      id.ClientID,
		TenantID:      id.TenantID,
		// The authority host only ever comes from the gateway's environment, so it needs no
		// validation by the public cloud, which an air-gapped or sovereign gateway can't reach.
		DisableInstanceDiscovery: true,
	})
}

// newManagedIdentityCredential builds the SDK credential for the host's managed identity, from
// whichever endpoint the host's environment points the SDK at.
func newManagedIdentityCredential(id Identity) (azcore.TokenCredential, error) {
	return managedIdentityCredential(id.ClientID, managedRetry)
}

// newContainerAppsCredential builds the SDK credential for a managed identity of the container app
// the gateway runs in. The SDK uses the Container Apps identity endpoint only while
// IDENTITY_ENDPOINT and IDENTITY_HEADER are set; otherwise it falls back to IMDS, which Container
// Apps doesn't have, and every request would wait out its timeout.
func newContainerAppsCredential(id Identity) (azcore.TokenCredential, error) {
	var missing []string
	for _, name := range []string{"IDENTITY_ENDPOINT", "IDENTITY_HEADER"} {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	switch {
	case len(missing) == 1:
		return nil, fmt.Errorf("%s is not set", missing[0])
	case len(missing) > 1:
		return nil, fmt.Errorf("%s are not set", listing(missing, "and"))
	case os.Getenv("IDENTITY_SERVER_THUMBPRINT") != "":
		return nil, errors.New("IDENTITY_SERVER_THUMBPRINT is set, which points the SDK at Service Fabric's endpoint instead")
	}
	return managedIdentityCredential(id.ClientID, containerAppsRetry)
}

// managedIdentityCredential builds the SDK credential for the host's managed identity clientID, or
// its default identity when clientID is "". It never reads AZURE_CLIENT_ID: the Workload Identity
// webhook sets it for the federated identity, which need not be assigned to the host.
func managedIdentityCredential(clientID string, retry azpolicy.RetryOptions) (azcore.TokenCredential, error) {
	opts := &azidentity.ManagedIdentityCredentialOptions{
		ClientOptions: azcore.ClientOptions{Retry: retry, Transport: transport},
	}
	if clientID != "" {
		opts.ID = azidentity.ClientID(clientID)
	}
	return azidentity.NewManagedIdentityCredential(opts)
}
