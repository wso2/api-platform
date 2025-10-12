package config

import "sync"
import "github.com/kelseyhightower/envconfig"

// Server holds the configuration parameters for the application.
type Server struct {
	LogLevel string `envconfig:"LOG_LEVEL" default:"DEBUG"`
	//Logger   logging.Logger
}

// package-level variable and mutex for thread safety
var (
	processOnce     sync.Once
	settingInstance *Server
)

// GetConfig initializes and returns a singleton instance of the Settings struct.
// It uses sync.Once to ensure that the initialization logic is executed only once,
// making it safe for concurrent use. If there is an error during the initialization,
// the function will panic.
//
// Returns:
//
//	*Settings - A pointer to the singleton instance of the Settings struct. from environment variables.
func GetConfig() *Server {
	var err error
	processOnce.Do(func() {
		settingInstance = &Server{}
		err = envconfig.Process("", settingInstance)
	})
	if err != nil {
		panic(err)
	}
	// Create Logger based on the env var
	//settingInstance.Logger = logging.NewLogger()
	return settingInstance
}
