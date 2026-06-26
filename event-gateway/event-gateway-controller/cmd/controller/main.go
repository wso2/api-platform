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

// Command event-gateway-controller starts the gateway controller with event-gateway
// extensions (WebSub APIs, WebBroker APIs, and HMAC webhook secrets) enabled.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/server"
	gwextension "github.com/wso2/api-platform/gateway/gateway-controller/pkg/extension"
	evextension "github.com/wso2/api-platform/event-gateway/event-gateway-controller/pkg/extension"
)

func main() {
	configPath := flag.String("config", "", "Path to the configuration file (required)")
	flag.Parse()
	if *configPath == "" {
		fmt.Fprintf(os.Stderr, "Error: -config flag is required\n")
		fmt.Fprintf(os.Stderr, "Usage: %s -config <path-to-config.toml>\n", os.Args[0])
		os.Exit(1)
	}
	server.Run(*configPath, []gwextension.Extension{evextension.NewEventGatewayExtension()})
}
