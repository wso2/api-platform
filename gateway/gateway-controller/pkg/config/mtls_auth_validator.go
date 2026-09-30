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

package config

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	api "github.com/wso2/api-platform/gateway/gateway-controller/pkg/api/management"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/clientca"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
)

// MtlsAuthPolicyName is the policy name this validator applies to.
const MtlsAuthPolicyName = "mtls-auth"

// Warning codes for the mtls-auth deploy-response warnings.
const (
	WarningCodeMTLSAcceptInheritsPool   = "MTLS_ACCEPT_INHERITS_POOL"
	WarningCodeMTLSAcceptUnnarrowed     = "MTLS_ACCEPT_UNNARROWED"
	WarningCodeMTLSAuthNotFirst         = "MTLS_AUTH_NOT_FIRST"
	WarningCodeMTLSThumbprintNormalised = "MTLS_THUMBPRINT_NORMALISED"

	// WarningCodeMTLSAcceptNamesRelayAuthority is raised for an accept entry
	// whose authority is also a relay entry's. Such an API authenticates the
	// front proxy itself and never a certificate it relays.
	WarningCodeMTLSAcceptNamesRelayAuthority = "MTLS_ACCEPT_NAMES_RELAY_AUTHORITY"

	// WarningCodeHeaderCertBypassActive is attached to every mtls-auth deploy
	// response while client_certificate_header.trust_any is true, because the
	// relayed-certificate header is then believed from any connection.
	WarningCodeHeaderCertBypassActive = "HEADER_CERT_BYPASS_ACTIVE"
)

// mtlsAuthPrecedingAuthPolicies lists other authentication-policy names
// that, when they appear earlier than mtls-auth in the same policy chain,
// trigger the MTLS_AUTH_NOT_FIRST warning.
var mtlsAuthPrecedingAuthPolicies = map[string]bool{
	"jwt-auth":          true,
	"api-key-auth":      true,
	"basic-auth":        true,
	"opaque-token-auth": true,
	"mcp-auth":          true,
	"oauth2-auth":       true,
}

var thumbprintHexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// MtlsAuthCertificateStore is the read-only subset of storage.Storage the
// mtls-auth validator needs to look up the client authority pool.
type MtlsAuthCertificateStore interface {
	GetCertificateByName(name string) (*models.StoredCertificate, error)
	ListCertificatesByUsage(usage string) ([]*models.StoredCertificate, error)
}

// MtlsAuthValidator validates deploy-time use of the mtls-auth policy and
// resolves the warnings and accept-list echo of a successful deploy. A
// deployment that could never authenticate anyone is refused, not deployed
// inert.
type MtlsAuthValidator struct {
	store          MtlsAuthCertificateStore
	httpsEnabled   bool
	headerTrustAny bool
	keys           mtlsAuthParamKeys
}

// NewMtlsAuthValidator creates a validator bound to the certificate store.
// httpsEnabled is router.https_enabled. headerTrustAny is
// client_certificate_header.trust_any; when true the HTTPS-listener
// requirement is relaxed, since a relayed header can arrive over plaintext.
// paramSchema is the mtls-auth definition's parameter schema, the source of
// the parameter names it accepts; nil reports no unknown parameters.
func NewMtlsAuthValidator(store MtlsAuthCertificateStore, httpsEnabled, headerTrustAny bool, paramSchema map[string]interface{}) *MtlsAuthValidator {
	return &MtlsAuthValidator{
		store:          store,
		httpsEnabled:   httpsEnabled,
		headerTrustAny: headerTrustAny,
		keys:           mtlsAuthParamKeysFromSchema(paramSchema),
	}
}

// MtlsAuthParameterSchema returns the parameter schema of the latest loaded
// mtls-auth definition, or nil when none is loaded.
func MtlsAuthParameterSchema(definitions map[string]models.PolicyDefinition) map[string]interface{} {
	version, err := ResolvePolicyVersion(definitions, nil, MtlsAuthPolicyName, "")
	if err != nil {
		return nil
	}
	def, ok := definitions[MtlsAuthPolicyName+"|"+version]
	if !ok || def.Parameters == nil {
		return nil
	}
	return *def.Parameters
}

// mtlsAuthParamKeys holds the parameter names the mtls-auth schema declares
// at each level an author writes keys. A nil set means the schema was not
// available, and that level reports no unknown parameters.
type mtlsAuthParamKeys struct {
	top         map[string]bool
	acceptEntry map[string]bool
	match       map[string]bool
}

func mtlsAuthParamKeysFromSchema(schema map[string]interface{}) mtlsAuthParamKeys {
	top := schemaProperties(schema)
	entry := schemaProperties(asSchema(asSchema(top["accept"])["items"]))
	return mtlsAuthParamKeys{
		top:         propertyNames(top),
		acceptEntry: propertyNames(entry),
		match:       propertyNames(schemaProperties(asSchema(entry["match"]))),
	}
}

func asSchema(v interface{}) map[string]interface{} {
	m, _ := v.(map[string]interface{})
	return m
}

func schemaProperties(schema map[string]interface{}) map[string]interface{} {
	props, _ := schema["properties"].(map[string]interface{})
	return props
}

func propertyNames(props map[string]interface{}) map[string]bool {
	if props == nil {
		return nil
	}
	names := make(map[string]bool, len(props))
	for name := range props {
		names[name] = true
	}
	return names
}

// unknownKeys reports every key of params absent from known, in no
// particular order. A nil known reports none.
func unknownKeys(params map[string]interface{}, known map[string]bool) []string {
	if known == nil {
		return nil
	}
	var unknown []string
	for key := range params {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	return unknown
}

// mtlsOccurrence is one place mtls-auth is attached in a RestAPI: either the
// API-level policies list or a single operation's policies list.
type mtlsOccurrence struct {
	fieldPath   string
	params      map[string]interface{}
	scopeKey    string // "api" or "op:<index>" — occurrences sharing a scopeKey are in the same chain
	apiLevel    bool
	conditional bool // an executionCondition is set, so the engine could skip the policy
}

func hasExecutionCondition(cond *string) bool {
	return cond != nil && strings.TrimSpace(*cond) != ""
}

func paramsOrEmpty(p *map[string]interface{}) map[string]interface{} {
	if p == nil {
		return map[string]interface{}{}
	}
	return *p
}

// collectMTLSAuthOccurrences finds every mtls-auth attachment at API and
// operation level, in document order.
func collectMTLSAuthOccurrences(apiConfig *api.RestAPI) []mtlsOccurrence {
	var occs []mtlsOccurrence

	if apiConfig.Spec.Policies != nil {
		for i, p := range *apiConfig.Spec.Policies {
			if p.Name == MtlsAuthPolicyName {
				occs = append(occs, mtlsOccurrence{
					fieldPath:   fmt.Sprintf("spec.policies[%d]", i),
					params:      paramsOrEmpty(p.Params),
					conditional: hasExecutionCondition(p.ExecutionCondition),
					scopeKey:    "api",
					apiLevel:    true,
				})
			}
		}
	}

	for opIdx, op := range apiConfig.Spec.Operations {
		if op.Policies == nil {
			continue
		}
		for pIdx, p := range *op.Policies {
			if p.Name == MtlsAuthPolicyName {
				occs = append(occs, mtlsOccurrence{
					fieldPath:   fmt.Sprintf("spec.operations[%d].policies[%d]", opIdx, pIdx),
					params:      paramsOrEmpty(p.Params),
					conditional: hasExecutionCondition(p.ExecutionCondition),
					scopeKey:    fmt.Sprintf("op:%d", opIdx),
					apiLevel:    false,
				})
			}
		}
	}

	return occs
}

// NamedAcceptEntryFieldPaths returns, in document order, the field path of
// every mtls-auth accept entry on apiConfig whose ca names caName. An omitted
// accept, which inherits the pool, contributes nothing.
func NamedAcceptEntryFieldPaths(apiConfig *api.RestAPI, caName string) []string {
	var paths []string
	for _, occ := range collectMTLSAuthOccurrences(apiConfig) {
		acceptRaw, hasAccept := occ.params["accept"]
		if !hasAccept {
			continue
		}
		acceptSlice, ok := acceptRaw.([]interface{})
		if !ok {
			continue
		}
		for j, entryRaw := range acceptSlice {
			entryMap, ok := entryRaw.(map[string]interface{})
			if !ok {
				continue
			}
			name, _ := entryMap["ca"].(string)
			if strings.TrimSpace(name) == caName {
				paths = append(paths, fmt.Sprintf("%s.params.accept[%d].ca", occ.fieldPath, j))
			}
		}
	}
	return paths
}

// MtlsAuthAttachmentFieldPaths returns the field path of every mtls-auth
// occurrence on apiConfig, in document order.
func MtlsAuthAttachmentFieldPaths(apiConfig *api.RestAPI) []string {
	occs := collectMTLSAuthOccurrences(apiConfig)
	paths := make([]string, 0, len(occs))
	for _, occ := range occs {
		paths = append(paths, occ.fieldPath)
	}
	return paths
}

// ValidateRestAPI reports every deploy-blocking problem with the mtls-auth
// attachments on apiConfig.
func (v *MtlsAuthValidator) ValidateRestAPI(apiConfig *api.RestAPI) []ValidationError {
	occs := collectMTLSAuthOccurrences(apiConfig)
	if len(occs) == 0 {
		return nil
	}

	var errs []ValidationError

	// Only one occurrence is allowed per policy chain.
	seenScope := map[string]bool{}
	for _, occ := range occs {
		if seenScope[occ.scopeKey] {
			errs = append(errs, ValidationError{
				Field:   occ.fieldPath,
				Message: "mtls-auth may appear once per scope; use several accept entries instead",
			})
		}
		seenScope[occ.scopeKey] = true
	}

	// Operation-level occurrences are refused when an API-level one exists.
	hasAPILevel := false
	for _, occ := range occs {
		if occ.apiLevel {
			hasAPILevel = true
			break
		}
	}
	if hasAPILevel {
		for _, occ := range occs {
			if !occ.apiLevel {
				errs = append(errs, ValidationError{
					Field:   occ.fieldPath,
					Message: "mtls-auth is already attached at API level; attach it at one level only",
				})
			}
		}
	}

	clientAuthorities, err := v.store.ListCertificatesByUsage(models.CertificateUsageDownstream)
	poolEmpty := err != nil || countClientAuthorities(clientAuthorities) == 0

	for _, occ := range occs {
		if occ.conditional {
			errs = append(errs, ValidationError{
				Field:   occ.fieldPath + ".executionCondition",
				Message: "mtls-auth runs on every request and cannot carry an executionCondition",
			})
		}

		if !v.httpsEnabled && !v.headerTrustAny {
			errs = append(errs, ValidationError{
				Field:   occ.fieldPath,
				Message: "mtls-auth requires the HTTPS listener, which is disabled on this gateway",
			})
		}

		if poolEmpty {
			errs = append(errs, ValidationError{
				Field:   occ.fieldPath,
				Message: "mtls-auth requires at least one client authority; add one with POST /certificates and usage: downstream",
			})
		}

		errs = append(errs, v.validateParams(occ.fieldPath, occ.params)...)
	}

	return errs
}

// validateParams validates what the parameter schema cannot say as well:
// unknown keys, the accept list's authorities and narrowing, and the shape
// of accept and forwardCertificate in the author's terms. A schema error at
// a field this reports on is dropped in its favour.
func (v *MtlsAuthValidator) validateParams(fieldPath string, params map[string]interface{}) []ValidationError {
	var errs []ValidationError
	paramsPath := fieldPath + ".params"

	for _, key := range unknownKeys(params, v.keys.top) {
		errs = append(errs, unknownParamError(paramsPath, key))
	}

	if forward, present := params["forwardCertificate"]; present {
		if _, isBool := forward.(bool); !isBool {
			errs = append(errs, ValidationError{
				Field:   paramsPath + ".forwardCertificate",
				Message: "forwardCertificate must be true or false",
			})
		}
	}

	acceptRaw, hasAccept := params["accept"]
	if !hasAccept {
		return errs
	}
	acceptPath := paramsPath + ".accept"
	acceptSlice, ok := acceptRaw.([]interface{})
	if !ok {
		errs = append(errs, ValidationError{
			Field:   acceptPath,
			Message: "accept must be a list of entries; omit it to inherit every pooled authority",
		})
		return errs
	}
	if len(acceptSlice) == 0 {
		errs = append(errs, ValidationError{
			Field:   acceptPath,
			Message: "omit accept to inherit every pooled authority, or list at least one entry",
		})
		return errs
	}

	for j, entryRaw := range acceptSlice {
		entryPath := fmt.Sprintf("%s[%d]", acceptPath, j)
		entryMap, ok := entryRaw.(map[string]interface{})
		if !ok {
			errs = append(errs, ValidationError{
				Field:   entryPath,
				Message: "each accept entry must be an object naming ca",
			})
			continue
		}
		errs = append(errs, v.validateAcceptEntry(entryPath, entryMap)...)
	}

	return errs
}

func (v *MtlsAuthValidator) validateAcceptEntry(entryPath string, entry map[string]interface{}) []ValidationError {
	var errs []ValidationError

	for _, key := range unknownKeys(entry, v.keys.acceptEntry) {
		if key == "thumbprint" {
			errs = append(errs, ValidationError{
				Field:   entryPath + ".thumbprint",
				Message: "unknown parameter thumbprint; the field is thumbprints",
			})
			continue
		}
		errs = append(errs, unknownParamError(entryPath, key))
	}

	errs = append(errs, v.validateAcceptEntryCA(entryPath, entry)...)
	errs = append(errs, v.validateAcceptEntryMatch(entryPath, entry)...)
	errs = append(errs, validateAcceptEntryThumbprints(entryPath, entry)...)

	return errs
}

func (v *MtlsAuthValidator) validateAcceptEntryCA(entryPath string, entry map[string]interface{}) []ValidationError {
	caPath := entryPath + ".ca"
	caRaw, hasCA := entry["ca"]
	caName, caIsString := caRaw.(string)
	if !hasCA || !caIsString || strings.TrimSpace(caName) == "" {
		return []ValidationError{{
			Field:   caPath,
			Message: "ca is required and must name an authority in this gateway's client-CA pool",
		}}
	}

	cert, err := v.store.GetCertificateByName(caName)
	if err != nil || cert == nil {
		return []ValidationError{{
			Field:   caPath,
			Message: fmt.Sprintf("no client-CA authority named %s exists on this gateway", caName),
		}}
	}

	switch cert.EffectiveUsage() {
	case models.CertificateUsageDownstream:
	case models.CertificateUsageUpstream:
		return []ValidationError{{
			Field:   caPath,
			Message: fmt.Sprintf("%s is a backend trust certificate (usage: upstream); accept takes usage: downstream authorities", caName),
		}}
	case models.CertificateUsageIdentity:
		return []ValidationError{{
			Field:   caPath,
			Message: fmt.Sprintf("%s is a gateway identity (usage: identity); accept takes usage: downstream authorities", caName),
		}}
	default:
		return []ValidationError{{
			Field:   caPath,
			Message: fmt.Sprintf("%s has an unrecognized usage; accept takes usage: downstream authorities", caName),
		}}
	}

	role := cert.EffectiveRole()
	if role == models.CertificateRoleRelay {
		return []ValidationError{{
			Field:   caPath,
			Message: fmt.Sprintf("%s is a relay (front proxy) entry and cannot be accepted as a client", caName),
		}}
	}

	return nil
}

func (v *MtlsAuthValidator) validateAcceptEntryMatch(entryPath string, entry map[string]interface{}) []ValidationError {
	matchRaw, hasMatch := entry["match"]
	if !hasMatch || matchRaw == nil {
		return nil
	}
	matchPath := entryPath + ".match"
	matchMap, ok := matchRaw.(map[string]interface{})
	if !ok {
		return []ValidationError{{
			Field:   matchPath,
			Message: "match must be an object listing uriSANs or dnsSANs",
		}}
	}

	var errs []ValidationError

	for _, key := range unknownKeys(matchMap, v.keys.match) {
		errs = append(errs, unknownParamError(matchPath, key))
	}

	_, hasURI := matchMap["uriSANs"]
	_, hasDNS := matchMap["dnsSANs"]
	if !hasURI && !hasDNS {
		errs = append(errs, ValidationError{
			Field:   matchPath,
			Message: "match must list uriSANs or dnsSANs; remove it to accept any certificate from this authority",
		})
		return errs
	}

	for _, sanField := range []string{"uriSANs", "dnsSANs"} {
		sanRaw, ok := matchMap[sanField]
		if !ok {
			continue
		}
		sanPath := matchPath + "." + sanField
		sanSlice, ok := sanRaw.([]interface{})
		if !ok {
			errs = append(errs, ValidationError{Field: sanPath, Message: sanField + " must be a list"})
			continue
		}
		const sanEmptyMessage = "list at least one non-empty SAN, or remove match to accept any certificate from this authority"
		if len(sanSlice) == 0 {
			errs = append(errs, ValidationError{Field: sanPath, Message: sanEmptyMessage})
			continue
		}
		for k, sv := range sanSlice {
			s, _ := sv.(string)
			if strings.TrimSpace(s) == "" {
				errs = append(errs, ValidationError{
					Field:   fmt.Sprintf("%s[%d]", sanPath, k),
					Message: sanEmptyMessage,
				})
			}
		}
	}

	return errs
}

func validateAcceptEntryThumbprints(entryPath string, entry map[string]interface{}) []ValidationError {
	tpRaw, hasTP := entry["thumbprints"]
	if !hasTP {
		return nil
	}
	tpPath := entryPath + ".thumbprints"
	tpSlice, ok := tpRaw.([]interface{})
	if !ok {
		return []ValidationError{{Field: tpPath, Message: "thumbprints must be a list"}}
	}

	var errs []ValidationError
	if len(tpSlice) == 0 {
		errs = append(errs, ValidationError{
			Field:   tpPath,
			Message: "list at least one thumbprint, or remove thumbprints to accept any certificate from this authority",
		})
		return errs
	}

	for k, tv := range tpSlice {
		s, _ := tv.(string)
		if _, _, valid := normalizeThumbprint(s); !valid {
			errs = append(errs, ValidationError{
				Field:   fmt.Sprintf("%s[%d]", tpPath, k),
				Message: "a thumbprint is the SHA-256 of the certificate as 64 hex characters (colons and a sha256: prefix are accepted)",
			})
		}
	}

	return errs
}

// normalizeThumbprint canonicalises a thumbprint to lowercase with no colons
// and no "sha256:" prefix. valid reports whether the result is 64 hex
// characters; changed reports whether the input was altered.
func normalizeThumbprint(raw string) (normalized string, changed bool, valid bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, "sha256:")
	s = strings.ReplaceAll(s, ":", "")
	if !thumbprintHexPattern.MatchString(s) {
		return "", false, false
	}
	return s, s != raw, true
}

func unknownParamError(basePath, key string) ValidationError {
	return ValidationError{
		Field:   basePath + "." + key,
		Message: "unknown parameter " + key,
	}
}

// clientAuthorityPool is the usage: downstream part of the certificate store: the
// non-relay authorities an omitted accept inherits, every entry by name, and
// the relay entries.
type clientAuthorityPool struct {
	names  []string
	byName map[string]*models.StoredCertificate
	relays []*models.StoredCertificate
}

// loadClientAuthorityPool reads every usage: downstream row once. A store error
// yields an empty pool: the deploy already validated, so the response then
// simply carries no pool-derived echo or warnings.
func (v *MtlsAuthValidator) loadClientAuthorityPool() clientAuthorityPool {
	pool := clientAuthorityPool{byName: map[string]*models.StoredCertificate{}}
	certs, err := v.store.ListCertificatesByUsage(models.CertificateUsageDownstream)
	if err != nil {
		return pool
	}
	for _, cert := range certs {
		pool.byName[cert.Name] = cert
		if cert.Role == models.CertificateRoleRelay {
			pool.relays = append(pool.relays, cert)
			continue
		}
		pool.names = append(pool.names, cert.Name)
	}
	return pool
}

// relaySharingAuthority returns the first relay entry that shares an
// authority with the named client entry, or nil. Two entries share an
// authority when their stored certificate bytes are identical or their
// identity certificates (the bottom of each stored chain) are equal.
func (p clientAuthorityPool) relaySharingAuthority(caName string) *models.StoredCertificate {
	client, ok := p.byName[caName]
	if !ok || len(client.Certificate) == 0 {
		return nil
	}
	clientIdentity, clientErr := clientca.IdentityCertificate(client.Certificate)
	for _, relay := range p.relays {
		if len(relay.Certificate) == 0 {
			continue
		}
		if bytes.Equal(client.Certificate, relay.Certificate) {
			return relay
		}
		if clientErr != nil {
			continue
		}
		if relayIdentity, err := clientca.IdentityCertificate(relay.Certificate); err == nil && clientIdentity.Equal(relayIdentity) {
			return relay
		}
	}
	return nil
}

// ResolveMtlsAuthForResponse returns a copy of apiConfig whose mtls-auth
// policies carry the resolved accept list, plus the deploy warnings. The
// stored configuration stays as sent so pool changes keep applying. Call it
// only after ValidateRestAPI reported no errors.
func (v *MtlsAuthValidator) ResolveMtlsAuthForResponse(apiConfig api.RestAPI) (api.RestAPI, []clientca.Warning) {
	if collectMTLSAuthOccurrences(&apiConfig) == nil {
		return apiConfig, nil
	}

	pool := v.loadClientAuthorityPool()

	var warnings []clientca.Warning

	if apiConfig.Spec.Policies != nil {
		resolved, w := v.resolvePolicyList(*apiConfig.Spec.Policies, "spec.policies", pool)
		apiConfig.Spec.Policies = &resolved
		warnings = append(warnings, w...)
	}

	newOps := make([]api.Operation, len(apiConfig.Spec.Operations))
	copy(newOps, apiConfig.Spec.Operations)
	for i := range newOps {
		if newOps[i].Policies == nil {
			continue
		}
		resolved, w := v.resolvePolicyList(*newOps[i].Policies, fmt.Sprintf("spec.operations[%d].policies", i), pool)
		newOps[i].Policies = &resolved
		warnings = append(warnings, w...)
	}
	apiConfig.Spec.Operations = newOps

	if v.headerTrustAny {
		warnings = append(warnings, clientca.Warning{
			Code:    WarningCodeHeaderCertBypassActive,
			Field:   "",
			Message: "the client certificate header is believed from any connection (trust_any = true)",
		})
	}

	return apiConfig, warnings
}

// resolvePolicyList resolves every mtls-auth entry in one policy chain and
// reports MTLS_AUTH_NOT_FIRST for any auth policy preceding it.
func (v *MtlsAuthValidator) resolvePolicyList(policies []api.Policy, listPath string, pool clientAuthorityPool) ([]api.Policy, []clientca.Warning) {
	resolved := make([]api.Policy, len(policies))
	copy(resolved, policies)

	var warnings []clientca.Warning
	mtlsSeen := false
	for i, p := range policies {
		if p.Name == MtlsAuthPolicyName {
			mtlsSeen = true
			continue
		}
		if mtlsAuthPrecedingAuthPolicies[p.Name] && !mtlsSeen {
			// Only warn once mtls-auth actually appears later in this chain.
			for _, later := range policies[i+1:] {
				if later.Name == MtlsAuthPolicyName {
					warnings = append(warnings, clientca.Warning{
						Code:    WarningCodeMTLSAuthNotFirst,
						Field:   fmt.Sprintf("%s[%d]", listPath, i),
						Message: fmt.Sprintf("%s runs before mtls-auth in this chain; an authentication policy ahead of mtls-auth runs first", p.Name),
					})
					break
				}
			}
		}
	}

	for i, p := range policies {
		if p.Name != MtlsAuthPolicyName {
			continue
		}
		fieldPath := fmt.Sprintf("%s[%d]", listPath, i)
		newPolicy, w := v.resolveOnePolicy(p, fieldPath, pool)
		resolved[i] = newPolicy
		warnings = append(warnings, w...)
	}

	return resolved, warnings
}

// resolveOnePolicy resolves one mtls-auth policy's accept list for the
// response echo and reports the accept-list warnings that apply.
func (v *MtlsAuthValidator) resolveOnePolicy(p api.Policy, fieldPath string, pool clientAuthorityPool) (api.Policy, []clientca.Warning) {
	params := paramsOrEmpty(p.Params)
	newParams := make(map[string]interface{}, len(params))
	for k, val := range params {
		newParams[k] = val
	}

	var warnings []clientca.Warning
	acceptPath := fieldPath + ".params.accept"

	acceptRaw, hasAccept := params["accept"]
	if !hasAccept {
		resolvedAccept := make([]interface{}, 0, len(pool.names))
		for _, name := range pool.names {
			resolvedAccept = append(resolvedAccept, map[string]interface{}{"ca": name})
		}
		newParams["accept"] = resolvedAccept
		if len(pool.names) > 1 {
			warnings = append(warnings, clientca.Warning{
				Code:  WarningCodeMTLSAcceptInheritsPool,
				Field: acceptPath,
				Message: fmt.Sprintf(
					"accept is omitted and the pool holds %d authorities; this API accepts certificates from all of them",
					len(pool.names)),
			})
		}
	} else if acceptSlice, ok := acceptRaw.([]interface{}); ok {
		newAccept := make([]interface{}, len(acceptSlice))
		for j, entryRaw := range acceptSlice {
			entryMap, ok := entryRaw.(map[string]interface{})
			if !ok {
				newAccept[j] = entryRaw
				continue
			}
			newEntry := make(map[string]interface{}, len(entryMap))
			for k, val := range entryMap {
				newEntry[k] = val
			}
			entryPath := fmt.Sprintf("%s[%d]", acceptPath, j)

			_, hasMatch := entryMap["match"]
			tpRaw, hasThumbprints := entryMap["thumbprints"]
			if !hasMatch && !hasThumbprints {
				warnings = append(warnings, clientca.Warning{
					Code:  WarningCodeMTLSAcceptUnnarrowed,
					Field: entryPath,
					Message: "this entry has neither match nor thumbprints, so it accepts any certificate from this authority; " +
						"narrow it with match or thumbprints",
				})
			}

			if caName, ok := entryMap["ca"].(string); ok {
				if relay := pool.relaySharingAuthority(caName); relay != nil {
					warnings = append(warnings, clientca.Warning{
						Code:  WarningCodeMTLSAcceptNamesRelayAuthority,
						Field: entryPath + ".ca",
						Message: fmt.Sprintf("%s is also pooled as the relay %s; this API authenticates that front proxy itself "+
							"and never evaluates a certificate it relays", caName, relay.Name),
					})
				}
			}

			if hasThumbprints {
				if tpSlice, ok := tpRaw.([]interface{}); ok {
					newTP := make([]interface{}, len(tpSlice))
					for k, tv := range tpSlice {
						s, _ := tv.(string)
						canonical, changed, valid := normalizeThumbprint(s)
						if !valid {
							newTP[k] = tv
							continue
						}
						newTP[k] = canonical
						if changed {
							warnings = append(warnings, clientca.Warning{
								Code:    WarningCodeMTLSThumbprintNormalised,
								Field:   fmt.Sprintf("%s.thumbprints[%d]", entryPath, k),
								Message: fmt.Sprintf("thumbprint normalised to %s", canonical),
							})
						}
					}
					newEntry["thumbprints"] = newTP
				}
			}

			newAccept[j] = newEntry
		}
		newParams["accept"] = newAccept
	}

	p.Params = &newParams
	return p, warnings
}

// ValidateMTLSStartupInvariant refuses to start when a persisted RestAPI
// attaches mtls-auth but neither HTTPS nor header trust_any is enabled, so
// such an API can never authenticate a caller. It applies the deploy-time
// check to configurations already stored.
func ValidateMTLSStartupInvariant(configs []*models.StoredConfig, httpsEnabled, headerTrustAny bool) error {
	if httpsEnabled || headerTrustAny {
		return nil
	}
	for _, cfg := range configs {
		restCfg, ok := cfg.Configuration.(api.RestAPI)
		if !ok {
			continue
		}
		if len(collectMTLSAuthOccurrences(&restCfg)) > 0 {
			return fmt.Errorf(
				"RestAPI %q attaches mtls-auth but router.https_enabled is false; "+
					"enable the HTTPS listener or remove mtls-auth before starting", cfg.Handle)
		}
	}
	return nil
}

// countClientAuthorities counts the pool rows that can vouch for a client
// directly. Relay rows only vouch for a certificate carried in a header, so
// a pool holding nothing but relays cannot authenticate anyone.
func countClientAuthorities(rows []*models.StoredCertificate) int {
	n := 0
	for _, row := range rows {
		if row.Role != models.CertificateRoleRelay {
			n++
		}
	}
	return n
}
