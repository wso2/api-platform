/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package v2alpha1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/wso2/api-platform/kubernetes/gateway-operator/api/v1"
)

// DefaultAdminUsername is the username used when spec.adminCredentials.username is
// empty. The CRD defaults it to the same value.
const DefaultAdminUsername = "admin"

var (
	secretNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	secretKeyPattern  = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)
	usernamePattern   = regexp.MustCompile(`^[^:\x00-\x1f\x7f]+$`)
)

// AdminCredentialsOf returns the spec.adminCredentials of a stored v1 APIGateway, read
// from its ConversionDataAnnotation, with the CRD defaults applied. It returns nil
// and no error when the gateway has none.
//
// The value is validated with the same rules the CRD enforces on v2alpha1, because a
// v1 or v1alpha1 client can write the annotation directly and bypass that
// validation. Any parse or validation error must fail the gateway closed: never fall
// back to other credentials.
func AdminCredentialsOf(gw *v1.APIGateway) (*GatewayAdminCredentials, error) {
	if gw == nil {
		return nil, nil
	}
	raw, ok := gw.Annotations[ConversionDataAnnotation]
	if !ok {
		return nil, nil
	}
	data, err := decodeConversionData(raw)
	if err != nil {
		return nil, err
	}
	creds := data.Spec.AdminCredentials
	if creds == nil {
		return nil, nil
	}
	creds = creds.DeepCopy()
	creds.Default()
	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("spec.adminCredentials: %w", err)
	}
	return creds, nil
}

// HasAdminCredentials reports whether a stored v1 APIGateway carries
// spec.adminCredentials, whether or not the value is valid. Use it to decide which
// credential path a gateway is on; AdminCredentialsOf reports whether that path can
// be used.
func HasAdminCredentials(gw *v1.APIGateway) bool {
	if gw == nil {
		return false
	}
	raw, ok := gw.Annotations[ConversionDataAnnotation]
	if !ok {
		return false
	}
	data, err := decodeConversionData(raw)
	if err != nil {
		// An unparseable annotation is treated as present, so the gateway fails
		// closed instead of silently switching to another credential.
		return true
	}
	return data.Spec.AdminCredentials != nil
}

// SetAdminCredentials sets (or, with nil, clears) spec.adminCredentials on a v1
// APIGateway the way the conversion webhook stores it. It is meant for tests and
// for in-process conversions.
func SetAdminCredentials(gw *v1.APIGateway, creds *GatewayAdminCredentials) {
	if creds == nil {
		delete(gw.Annotations, ConversionDataAnnotation)
		return
	}
	raw, err := json.Marshal(conversionData{Spec: conversionDataSpec{AdminCredentials: creds}})
	if err != nil {
		// GatewayAdminCredentials holds only strings and a bool pointer; it always marshals.
		panic(fmt.Sprintf("marshal %s: %v", ConversionDataAnnotation, err))
	}
	if gw.Annotations == nil {
		gw.Annotations = map[string]string{}
	}
	gw.Annotations[ConversionDataAnnotation] = string(raw)
}

// Default applies the CRD defaults: username "admin" and source "file".
func (c *GatewayAdminCredentials) Default() {
	if c.Username == "" {
		c.Username = DefaultAdminUsername
	}
	if c.Source == "" {
		c.Source = AdminCredentialsSourceFile
	}
}

// Validate applies the CRD's validation rules for spec.adminCredentials. Call it
// after Default. The messages name fields, never the Secret's contents.
func (c *GatewayAdminCredentials) Validate() error {
	if len(c.Username) == 0 || len(c.Username) > 253 || !usernamePattern.MatchString(c.Username) {
		return fmt.Errorf("username must be 1-253 characters without ':' or control characters")
	}
	if c.Source != AdminCredentialsSourceEnv && c.Source != AdminCredentialsSourceFile {
		return fmt.Errorf("source must be %q or %q", AdminCredentialsSourceEnv, AdminCredentialsSourceFile)
	}
	ref := c.PasswordSecretRef
	if ref.Name == "" {
		return fmt.Errorf("passwordSecretRef.name must name the Secret that holds the admin password")
	}
	if len(ref.Name) > 253 || !secretNamePattern.MatchString(ref.Name) {
		return fmt.Errorf("passwordSecretRef.name must be a Secret name: lowercase letters, digits, '-' and '.', at most 253 characters")
	}
	if len(ref.Key) > 253 || !secretKeyPattern.MatchString(ref.Key) || ref.Key == "." || strings.HasPrefix(ref.Key, "..") {
		return fmt.Errorf("passwordSecretRef.key must be a Secret key name (letters, digits, '-', '_', '.'), at most 253 characters, not starting with '..'")
	}
	if c.Source == AdminCredentialsSourceFile && strings.Contains(ref.Key, "..") {
		return fmt.Errorf("passwordSecretRef.key must not contain '..' when source is file")
	}
	if ref.Optional != nil && *ref.Optional {
		return fmt.Errorf("passwordSecretRef.optional must not be true: the admin password Secret is required")
	}
	return nil
}
