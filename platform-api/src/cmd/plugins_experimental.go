//go:build experimental

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

// Package main — experimental build: registers experimental plugins.
// Compiled only when the "experimental" build tag is set, e.g.:
//
//	go build -tags experimental ./cmd/main.go
//	docker build --build-arg EXPERIMENTAL=true ...
package main

import (
	"platform-api/src/internal/plugin"
	eventgateway "platform-api/src/plugins/eventgateway"
)

func init() {
	plugin.Register(eventgateway.New())
}
