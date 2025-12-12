/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package main

import (
	"log"
	"platform-api/src/config"
	"platform-api/src/internal/server"
)

func main() {
	cfg := config.GetConfig()

	// CreateOrganization and start server
	srv, err := server.StartPlatformAPIServer(cfg)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	log.Println("Starting HTTPS server on port 9243...")
	if err := srv.Start(":9243"); err != nil {
		log.Fatal("Failed to start HTTPS server:", err)
	}
}
