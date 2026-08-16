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

// Command testbench runs the mock services used by the integration suites.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/wso2/api-platform/tests/framework/testbench"
	"github.com/wso2/api-platform/tests/framework/testbench/services/analytics"
	"github.com/wso2/api-platform/tests/framework/testbench/services/backend"
	"github.com/wso2/api-platform/tests/framework/testbench/services/bedrock"
	"github.com/wso2/api-platform/tests/framework/testbench/services/contentsafety"
	"github.com/wso2/api-platform/tests/framework/testbench/services/echo"
	"github.com/wso2/api-platform/tests/framework/testbench/services/embeddings"
	"github.com/wso2/api-platform/tests/framework/testbench/services/interceptor"
	"github.com/wso2/api-platform/tests/framework/testbench/services/jwks"
	"github.com/wso2/api-platform/tests/framework/testbench/services/mcp"
	"github.com/wso2/api-platform/tests/framework/testbench/services/openai"
)

func main() {
	if err := run(); err != nil {
		slog.Error("testbench stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	log, err := newLogger()
	if err != nil {
		return err
	}
	reg := &testbench.Registry{}

	services, err := services()
	if err != nil {
		return err
	}

	for _, svc := range services {
		if err := reg.Register(svc); err != nil {
			return fmt.Errorf("registering service %q: %w", svc.Name(), err)
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	return testbench.Serve(ctx, reg, log)
}

func services() ([]testbench.Service, error) {
	jwksSvc, err := jwks.New()
	if err != nil {
		return nil, fmt.Errorf("building jwks service: %w", err)
	}
	return []testbench.Service{
		jwksSvc,
		echo.New(),
		backend.New(),
		bedrock.New(),
		openai.New(),
		interceptor.New(),
		mcp.New(),
		embeddings.New(),
		contentsafety.New(),
		analytics.New(),
	}, nil
}

func newLogger() (*slog.Logger, error) {
	level := new(slog.LevelVar)
	switch strings.ToLower(os.Getenv("TESTBENCH_LOG_LEVEL")) {
	case "", "info":
		level.Set(slog.LevelInfo)
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn", "warning":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		return nil, fmt.Errorf("invalid TESTBENCH_LOG_LEVEL %q", os.Getenv("TESTBENCH_LOG_LEVEL"))
	}

	options := &slog.HandlerOptions{Level: level}
	switch strings.ToLower(os.Getenv("TESTBENCH_LOG_FORMAT")) {
	case "", "text":
		return slog.New(slog.NewTextHandler(os.Stdout, options)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, options)), nil
	default:
		return nil, fmt.Errorf("invalid TESTBENCH_LOG_FORMAT %q", os.Getenv("TESTBENCH_LOG_FORMAT"))
	}
}
