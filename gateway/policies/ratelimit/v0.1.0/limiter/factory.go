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
