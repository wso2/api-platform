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

package service

// passthroughIdentityRepo is a repository.UserIdentityMappingRepository test
// double that returns the input identity unchanged (no real
// user_idp_references table). It lets unit tests that assert exact
// createdBy/updatedBy values (e.g. "alice") keep passing without needing a
// real DB-backed mapping.
type passthroughIdentityRepo struct{}

func (passthroughIdentityRepo) GetOrCreateUUID(identity string) (string, error) {
	return identity, nil
}

func (passthroughIdentityRepo) GetSubByUUID(uuid string) (string, bool, error) {
	if uuid == "" {
		return "", false, nil
	}
	return uuid, true, nil
}

func (passthroughIdentityRepo) GetSubsByUUIDs(uuids []string) (map[string]string, error) {
	result := make(map[string]string, len(uuids))
	for _, id := range uuids {
		if id != "" {
			result[id] = id
		}
	}
	return result, nil
}

// newTestIdentityService returns an *IdentityService backed by
// passthroughIdentityRepo, for unit tests that need to satisfy the
// IdentityService constructor param without a real database.
func newTestIdentityService() *IdentityService {
	return NewIdentityService(passthroughIdentityRepo{})
}
