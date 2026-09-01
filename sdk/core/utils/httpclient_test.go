/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
	"net/http"
	"sync"
	"testing"
)

func TestSharedHTTPClientNilBeforeSet(t *testing.T) {
	t.Cleanup(func() { SetSharedHTTPClient(nil) })

	SetSharedHTTPClient(nil)
	if got := SharedHTTPClient(); got != nil {
		t.Fatalf("expected nil before SetSharedHTTPClient, got %v", got)
	}
}

func TestSetSharedHTTPClientThenRetrieve(t *testing.T) {
	t.Cleanup(func() { SetSharedHTTPClient(nil) })

	client := &http.Client{}
	SetSharedHTTPClient(client)

	if got := SharedHTTPClient(); got != client {
		t.Fatalf("expected SharedHTTPClient to return the installed client, got %v", got)
	}
}

func TestSetSharedHTTPClientOverwritesPrevious(t *testing.T) {
	t.Cleanup(func() { SetSharedHTTPClient(nil) })

	first := &http.Client{}
	second := &http.Client{}

	SetSharedHTTPClient(first)
	SetSharedHTTPClient(second)

	if got := SharedHTTPClient(); got != second {
		t.Fatalf("expected the most recently installed client, got %v want %v", got, second)
	}
}

func TestSharedHTTPClientConcurrentAccess(t *testing.T) {
	t.Cleanup(func() { SetSharedHTTPClient(nil) })

	client := &http.Client{}
	SetSharedHTTPClient(client)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := SharedHTTPClient(); got != client {
				t.Errorf("expected consistent client under concurrent reads, got %v", got)
			}
		}()
	}
	wg.Wait()
}
