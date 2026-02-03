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
 
package limiter

import (
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config holds configuration for creating a rate limiter
type Config struct {
	Algorithm       string
	Limits          []LimitConfig
	Backend         string // "memory" or "redis"
	RedisClient     redis.UniversalClient
	KeyPrefix       string
	CleanupInterval time.Duration
	AlgorithmConfig map[string]interface{}
}

// AlgorithmFactory is a function that creates a limiter for a specific algorithm
type AlgorithmFactory func(config Config) (Limiter, error)

// algorithms holds registered algorithm factories
var algorithms = make(map[string]AlgorithmFactory)

// RegisterAlgorithm registers a new rate limiting algorithm
func RegisterAlgorithm(name string, factory AlgorithmFactory) {
	algorithms[name] = factory
}

// CreateLimiter creates a rate limiter based on the algorithm specified in config
func CreateLimiter(config Config) (Limiter, error) {
	factory, ok := algorithms[config.Algorithm]
	if !ok {
		return nil, fmt.Errorf("unknown algorithm: %s", config.Algorithm)
	}
	return factory(config)
}

// GetSupportedAlgorithms returns a list of registered algorithms
func GetSupportedAlgorithms() []string {
	algos := make([]string, 0, len(algorithms))
	for name := range algorithms {
		algos = append(algos, name)
	}
	return algos
}
