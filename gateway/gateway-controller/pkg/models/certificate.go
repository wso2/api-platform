/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package models

import "time"

// StoredCertificate represents a certificate stored in the database
type StoredCertificate struct {
	ID          string    `json:"id"`          // Unique ID (UUID)
	Name        string    `json:"name"`        // Human-readable name
	Certificate []byte    `json:"certificate"` // PEM-encoded certificate(s)
	Subject     string    `json:"subject"`     // Certificate subject DN
	Issuer      string    `json:"issuer"`      // Certificate issuer DN
	NotBefore   time.Time `json:"notBefore"`   // Certificate validity start
	NotAfter    time.Time `json:"notAfter"`    // Certificate validity end
	CertCount   int       `json:"certCount"`   // Number of certs in bundle
	CreatedAt   time.Time `json:"createdAt"`   // When uploaded
	UpdatedAt   time.Time `json:"updatedAt"`   // Last modified
}
