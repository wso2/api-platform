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

package model

import (
	"strings"
	"time"

	"github.com/wso2/api-platform/platform-api/internal/constants"
)

const (
	ServiceAccountStatusActive   = "active"
	ServiceAccountStatusDisabled = "disabled"

	ServiceAccountSubPrefix = constants.ServiceAccountSubPrefix

	// ServiceAccountSecretPrefix lets secret scanners recognise a leaked secret.
	// It is part of the secret: hashed with it, never stripped.
	ServiceAccountSecretPrefix = "apsa_"
)

// ServiceAccount is a non-human identity owned by an organization.
type ServiceAccount struct {
	UUID                string     `db:"uuid"`
	OrganizationID      string     `db:"organization_uuid"`
	Handle              string     `db:"handle"`
	DisplayName         string     `db:"name"`
	Version             string     `db:"version"`
	Description         string     `db:"description"`
	ClientID            string     `db:"client_id"`
	ClientSecretHash    string     `db:"client_secret_hash"`
	MaskedSecret        string     `db:"masked_secret"`
	Roles               string     `db:"roles"` // space-separated roles, expanded at each exchange
	Status              string     `db:"status"`
	TokenVersion        int64      `db:"token_version"` // bumped by every revoke; each token carries it
	LastUsedAt          *time.Time `db:"last_used_at"`
	LastUsedIP          string     `db:"last_used_ip"`
	SecretRegeneratedAt *time.Time `db:"secret_regenerated_at"`
	SecretRegeneratedBy string     `db:"secret_regenerated_by"`
	CreatedAt           time.Time  `db:"created_at"`
	CreatedBy           string     `db:"created_by"`
	UpdatedAt           time.Time  `db:"updated_at"`
	UpdatedBy           string     `db:"updated_by"`
}

// ServiceAccountRevocation is one account's watermark: tokens carrying a
// token version below MinTokenVersion are rejected.
type ServiceAccountRevocation struct {
	AccountUUID     string    `db:"account_uuid"`
	OrganizationID  string    `db:"organization_uuid"`
	MinTokenVersion int64     `db:"min_token_version"`
	ExpiresAt       time.Time `db:"expires_at"`
	RevokedBy       string    `db:"revoked_by"`
	RevokedAt       time.Time `db:"revoked_at"`
}

// RoleList returns the account's roles as a slice.
func (sa *ServiceAccount) RoleList() []string {
	return strings.Fields(sa.Roles)
}

// Subject is the token's sub: sa:<org-handle>:<handle>:<account-uuid>. The UUID
// keeps it unique forever, so a re-created handle never inherits an old identity.
func (sa *ServiceAccount) Subject(orgHandle string) string {
	return ServiceAccountSubPrefix + orgHandle + ":" + sa.Handle + ":" + sa.UUID
}

// AccountUUIDFromSubject returns the last segment of an sa: subject, or false.
func AccountUUIDFromSubject(sub string) (string, bool) {
	if !strings.HasPrefix(sub, ServiceAccountSubPrefix) {
		return "", false
	}
	i := strings.LastIndex(sub, ":")
	if i < len(ServiceAccountSubPrefix) || i == len(sub)-1 {
		return "", false
	}
	return sub[i+1:], true
}
