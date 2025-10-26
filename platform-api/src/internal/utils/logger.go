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

package utils

import (
	"fmt"
	"log"
	"runtime/debug"
)

// LogError logs an error with stack trace information
func LogError(message string, err error) {
	if err != nil {
		log.Printf("[ERROR] %s: %v\n", message, err)
		log.Printf("[STACK] %s\n", debug.Stack())
	}
}

// LogErrorWithContext logs an error with additional context and stack trace
func LogErrorWithContext(message string, err error, context map[string]interface{}) {
	if err != nil {
		contextStr := ""
		for k, v := range context {
			contextStr += fmt.Sprintf("%s=%v ", k, v)
		}
		log.Printf("[ERROR] %s: %v | Context: %s\n", message, err, contextStr)
		log.Printf("[STACK] %s\n", debug.Stack())
	}
}

// LogInfo logs informational messages
func LogInfo(message string) {
	log.Printf("[INFO] %s\n", message)
}

// LogWarning logs warning messages
func LogWarning(message string) {
	log.Printf("[WARN] %s\n", message)
}
