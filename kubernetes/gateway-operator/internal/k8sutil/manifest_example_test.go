/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8sutil_test

import (
	"context"
	"testing"

	"github.com/wso2/api-platform/kubernetes/gateway-operator/internal/k8sutil"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// Example demonstrates how to use the ManifestApplier
func ExampleManifestApplier() {
	// Create a fake client for testing
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create the manifest applier
	applier := k8sutil.NewManifestApplier(client, scheme)

	// Apply a manifest file
	ctx := context.Background()
	manifestPath := "path/to/your/manifest.yaml"
	namespace := "default"

	// owner can be nil if you don't want to set owner references
	err := applier.ApplyManifestFile(ctx, manifestPath, namespace, nil)
	if err != nil {
		// Handle error
		panic(err)
	}

	// Resources from the manifest are now created/updated in the cluster
}

// TestManifestApplier shows a unit test pattern
func TestManifestApplier(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewClientBuilder().WithScheme(scheme).Build()

	applier := k8sutil.NewManifestApplier(client, scheme)

	// Test applying a manifest
	ctx := context.Background()

	// In a real test, you would:
	// 1. Create a temporary manifest file
	// 2. Apply it using the applier
	// 3. Verify the resources were created
	// 4. Clean up

	_ = applier
	_ = ctx
}
