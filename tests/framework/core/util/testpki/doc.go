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

// Package testpki generates the certificate fixtures mutual TLS scenarios present and pool:
// authorities, intermediates, client leaves and gateway identities, each addressed by a
// fixture name. Several fixtures are defined relative to the generation time (expired, not
// yet valid, expiring soon), so the set is generated in memory once per process and never
// written to disk. Every key is throwaway test material.
package testpki
