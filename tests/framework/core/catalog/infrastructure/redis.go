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

package infrastructure

import (
	"github.com/wso2/api-platform/tests/framework/core/catalog/shared"
	"github.com/wso2/api-platform/tests/framework/core/components"
)

// RedisPasswordEnv is the environment variable carrying the Redis test password.
const RedisPasswordEnv = "REDIS_PASSWORD"

// RedisPassword is generated once for the framework process and shared by Redis callers.
var RedisPassword = mustRedisPassword()

func mustRedisPassword() string {
	password, err := shared.HexKey(32)
	if err != nil {
		panic("catalog: generating Redis password: " + err.Error())
	}
	return password
}

// Redis returns the Redis component used by suites that exercise shared caching or rate limits.
func Redis() *components.Definition {
	return &components.Definition{
		Name:  "redis",
		Alias: "redis",
		Image: components.ImageRef{Ref: "redis/redis-stack-server:latest"},
		Env:   map[string]string{RedisPasswordEnv: RedisPassword},
		Endpoints: []components.Endpoint{
			{Name: "redis", Port: 6379, Scheme: "tcp", AwaitListening: true},
		},
		Cmd:    []string{"redis-stack-server", "--requirepass", RedisPassword},
		Limits: components.ResourceLimits{CPUs: 0.5, MemoryMB: 512},
	}
}
