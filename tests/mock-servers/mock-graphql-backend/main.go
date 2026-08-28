// Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package main

import (
	"io"
	"log"
	"net/http"
	"strconv"
)

// handleGraphQL echoes the raw request body back verbatim as the response body.
//
// This stands in for a real GraphQL server in E2E tests that need to assert on the
// gateway's response-phase analytics enrichment: the shared sample-service fixture
// always wraps every response in a fixed {method,path,query,headers,body} envelope,
// so it can never produce a literal top-level "errors" array the way a real GraphQL
// server does. Echoing the request body verbatim lets a test fully control the
// response shape (including a GraphQL-style {"data":...,"errors":[...]} body) simply
// by choosing what it sends as the request.
func handleGraphQL(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	if codeStr := r.URL.Query().Get("statusCode"); codeStr != "" {
		if code, err := strconv.Atoi(codeStr); err == nil && code >= 100 && code <= 999 {
			w.WriteHeader(code)
		}
	}

	log.Printf("Mock GraphQL Backend: echoing request body (%d bytes)", len(body))
	w.Write(body)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/", handleGraphQL)

	log.Println("Mock GraphQL Backend listening on :8080")
	log.Println("Endpoints:")
	log.Println("  ANY  /*      - echoes the request body back verbatim as the response body")
	log.Println("  GET  /health - health check")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}
