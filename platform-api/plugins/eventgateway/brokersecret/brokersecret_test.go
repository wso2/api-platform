/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com).
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
 *
 */

package brokersecret

import (
	"reflect"
	"testing"
)

func TestBlankRedactsCredentialsAndKeepsEverythingElse(t *testing.T) {
	properties := map[string]interface{}{
		"brokers":        []interface{}{"kafka:9092"},
		"sasl_mechanism": "plain",
		"sasl_username":  "app-user",
		"sasl_password":  "s3cret",
	}

	got := Blank("kafka", properties)

	if got["sasl_password"] != "" {
		t.Fatalf("sasl_password = %v, want blank", got["sasl_password"])
	}
	if _, ok := got["sasl_password"]; !ok {
		t.Fatal("sasl_password key was dropped; it must stay so Preserve can read it back")
	}
	if got["sasl_username"] != "app-user" {
		t.Fatalf("sasl_username = %v, want it left visible", got["sasl_username"])
	}
	if got["sasl_mechanism"] != "plain" {
		t.Fatalf("sasl_mechanism = %v, want it left visible", got["sasl_mechanism"])
	}
	if properties["sasl_password"] != "s3cret" {
		t.Fatal("Blank modified the input map; callers hold a pointer into the stored model")
	}
}

func TestBlankCoversDottedKeysAndKeepsListShape(t *testing.T) {
	// The Choreo console writes dotted Kafka client property names, each value a
	// single-element list.
	properties := map[string]interface{}{
		"brokers":           []interface{}{"kafka:9092"},
		"security.protocol": []interface{}{"SASL_PLAINTEXT"},
		"sasl.username":     []interface{}{"app-user"},
		"sasl.password":     []interface{}{"s3cret"},
		"ssl.keystore.key":  []interface{}{"-----BEGIN PRIVATE KEY-----"},
		"ssl.key.password":  []interface{}{"passphrase"},
	}

	got := Blank("KAFKA", properties)

	for _, key := range []string{"sasl.password", "ssl.keystore.key", "ssl.key.password"} {
		want := []interface{}{""}
		if !reflect.DeepEqual(got[key], want) {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
	}
	if !reflect.DeepEqual(got["sasl.username"], []interface{}{"app-user"}) {
		t.Fatalf("sasl.username = %#v, want it left visible", got["sasl.username"])
	}
}

func TestBlankLeavesUnknownBrokerTypesAlone(t *testing.T) {
	properties := map[string]interface{}{"sasl_password": "s3cret"}

	if got := Blank("solace", properties); got["sasl_password"] != "s3cret" {
		t.Fatalf("sasl_password = %v, want it untouched for a type that declares no keys", got["sasl_password"])
	}
}

func TestPreserveKeepsStoredCredentialWhenIncomingIsBlank(t *testing.T) {
	stored := map[string]interface{}{"sasl_password": "s3cret", "sasl_username": "app-user"}
	incoming := map[string]interface{}{"sasl_password": "", "sasl_username": "new-user"}

	got := Preserve("kafka", incoming, stored)

	if got["sasl_password"] != "s3cret" {
		t.Fatalf("sasl_password = %v, want the stored value preserved", got["sasl_password"])
	}
	if got["sasl_username"] != "new-user" {
		t.Fatalf("sasl_username = %v, want the incoming value to win", got["sasl_username"])
	}
}

func TestPreserveReadsABlankListAsUnchanged(t *testing.T) {
	stored := map[string]interface{}{"sasl.password": []interface{}{"s3cret"}}
	incoming := map[string]interface{}{"sasl.password": []interface{}{""}}

	got := Preserve("kafka", incoming, stored)

	if !reflect.DeepEqual(got["sasl.password"], []interface{}{"s3cret"}) {
		t.Fatalf("sasl.password = %#v, want the stored value preserved", got["sasl.password"])
	}
}

func TestPreserveTakesANewCredential(t *testing.T) {
	stored := map[string]interface{}{"sasl_password": "old"}
	incoming := map[string]interface{}{"sasl_password": "new"}

	if got := Preserve("kafka", incoming, stored); got["sasl_password"] != "new" {
		t.Fatalf("sasl_password = %v, want the incoming value", got["sasl_password"])
	}
}

func TestPreserveClearsOnExplicitNull(t *testing.T) {
	stored := map[string]interface{}{"sasl_password": "s3cret"}
	incoming := map[string]interface{}{"sasl_password": nil}

	got := Preserve("kafka", incoming, stored)

	if _, ok := got["sasl_password"]; ok {
		t.Fatalf("sasl_password = %v, want it cleared by an explicit null", got["sasl_password"])
	}
}

func TestPreserveDropsABlankCredentialThatWasNeverStored(t *testing.T) {
	incoming := map[string]interface{}{"sasl_password": ""}

	got := Preserve("kafka", incoming, map[string]interface{}{})

	if _, ok := got["sasl_password"]; ok {
		t.Fatal("sasl_password should not be written when there is nothing to preserve")
	}
}

func TestPreserveLeavesTheInputMapAlone(t *testing.T) {
	stored := map[string]interface{}{"sasl_password": "s3cret"}
	incoming := map[string]interface{}{"sasl_password": ""}

	Preserve("kafka", incoming, stored)

	if incoming["sasl_password"] != "" {
		t.Fatal("Preserve modified the incoming map")
	}
}

func TestBlankThenPreserveRoundTripsUnchanged(t *testing.T) {
	stored := map[string]interface{}{
		"brokers":       []interface{}{"kafka:9092"},
		"sasl_username": "app-user",
		"sasl_password": "s3cret",
	}

	// What a client reads, edits elsewhere, and writes back.
	readBack := Blank("kafka", stored)
	readBack["sasl_username"] = "renamed"

	got := Preserve("kafka", readBack, stored)

	if got["sasl_password"] != "s3cret" {
		t.Fatalf("sasl_password = %v, want a read-then-write to leave it intact", got["sasl_password"])
	}
	if got["sasl_username"] != "renamed" {
		t.Fatalf("sasl_username = %v, want the edit to apply", got["sasl_username"])
	}
}
