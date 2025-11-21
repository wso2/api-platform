/*
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	core "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoverygrpc "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	runtimeservice "github.com/envoyproxy/go-control-plane/envoy/service/runtime/v3"
	"github.com/envoyproxy/go-control-plane/pkg/resource/v3"
	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	serverAddress = "localhost:18001"
	nodeID        = "policy-node"
)

func main() {
	// Initialize logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	logger.Info("Starting Policy xDS Client Example")

	// Create gRPC connection
	conn, err := grpc.Dial(
		serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(10*time.Second),
	)
	if err != nil {
		logger.Fatal("Failed to connect to xDS server",
			zap.String("address", serverAddress),
			zap.Error(err))
	}
	defer conn.Close()

	logger.Info("Successfully connected to xDS server", zap.String("address", serverAddress))

	// Create runtime discovery service client
	runtimeClient := runtimeservice.NewRuntimeDiscoveryServiceClient(conn)

	// Also create ADS client for demonstration
	adsClient := discoverygrpc.NewAggregatedDiscoveryServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start runtime stream
	go streamRuntimeUpdates(ctx, runtimeClient, logger)

	// Start ADS stream
	go streamADSUpdates(ctx, adsClient, logger)

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("Shutting down client...")
}

func streamRuntimeUpdates(ctx context.Context, client runtimeservice.RuntimeDiscoveryServiceClient, logger *zap.Logger) {
	logger.Info("Starting Runtime Discovery Service stream")

	stream, err := client.StreamRuntime(ctx)
	if err != nil {
		logger.Error("Failed to create runtime stream", zap.Error(err))
		return
	}

	// Send initial discovery request
	req := &discoverygrpc.DiscoveryRequest{
		Node: &core.Node{
			Id: nodeID,
		},
		TypeUrl: resource.RuntimeType,
	}

	if err := stream.Send(req); err != nil {
		logger.Error("Failed to send initial request", zap.Error(err))
		return
	}

	logger.Info("Sent initial runtime discovery request", zap.String("node_id", nodeID))

	// Receive and process responses
	responseCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("Runtime stream context cancelled")
			return
		default:
			resp, err := stream.Recv()
			if err != nil {
				logger.Error("Error receiving runtime response", zap.Error(err))
				return
			}

			responseCount++

			// Print a nice header
			printSeparator()
			fmt.Printf("📦 RECEIVED RUNTIME DISCOVERY RESPONSE #%d\n", responseCount)
			printSeparator()
			fmt.Printf("Version:        %s\n", resp.VersionInfo)
			fmt.Printf("Type URL:       %s\n", resp.TypeUrl)
			fmt.Printf("Nonce:          %s\n", resp.Nonce)
			fmt.Printf("Resource Count: %d\n", len(resp.Resources))
			printSeparator()

			logger.Info("Received runtime discovery response",
				zap.Int("response_count", responseCount),
				zap.String("version", resp.VersionInfo),
				zap.Int("resource_count", len(resp.Resources)),
			)

			// Process each runtime resource
			for i, res := range resp.Resources {
				runtime := &runtimeservice.Runtime{}
				if err := res.UnmarshalTo(runtime); err != nil {
					logger.Error("Failed to unmarshal runtime resource", zap.Error(err))
					continue
				}

				// Print detailed policy information
				printPolicyResource(i+1, runtime)
			}

			// Send ACK
			ackReq := &discoverygrpc.DiscoveryRequest{
				Node: &core.Node{
					Id: nodeID,
				},
				TypeUrl:       resp.TypeUrl,
				VersionInfo:   resp.VersionInfo,
				ResponseNonce: resp.Nonce,
			}

			if err := stream.Send(ackReq); err != nil {
				logger.Error("Failed to send ACK", zap.Error(err))
				return
			}

			logger.Info("Sent ACK for version", zap.String("version", resp.VersionInfo))
		}
	}
}

func streamADSUpdates(ctx context.Context, client discoverygrpc.AggregatedDiscoveryServiceClient, logger *zap.Logger) {
	logger.Info("Starting Aggregated Discovery Service stream")

	stream, err := client.StreamAggregatedResources(ctx)
	if err != nil {
		logger.Error("Failed to create ADS stream", zap.Error(err))
		return
	}

	// Send initial discovery request for runtime resources
	req := &discoverygrpc.DiscoveryRequest{
		Node: &core.Node{
			Id: nodeID,
		},
		TypeUrl: resource.RuntimeType,
	}

	if err := stream.Send(req); err != nil {
		logger.Error("Failed to send initial ADS request", zap.Error(err))
		return
	}

	logger.Info("Sent initial ADS discovery request", zap.String("node_id", nodeID))

	// Receive and process responses
	adsResponseCount := 0
	for {
		select {
		case <-ctx.Done():
			logger.Info("ADS stream context cancelled")
			return
		default:
			resp, err := stream.Recv()
			if err != nil {
				logger.Error("Error receiving ADS response", zap.Error(err))
				return
			}

			adsResponseCount++

			// Print ADS response summary
			fmt.Printf("\n🔄 ADS Response #%d | Version: %s | Resources: %d\n",
				adsResponseCount, resp.VersionInfo, len(resp.Resources))

			logger.Info("Received ADS discovery response",
				zap.Int("ads_response_count", adsResponseCount),
				zap.String("version", resp.VersionInfo),
				zap.Int("resource_count", len(resp.Resources)),
			)

			// Send ACK
			ackReq := &discoverygrpc.DiscoveryRequest{
				Node: &core.Node{
					Id: nodeID,
				},
				TypeUrl:       resp.TypeUrl,
				VersionInfo:   resp.VersionInfo,
				ResponseNonce: resp.Nonce,
			}

			if err := stream.Send(ackReq); err != nil {
				logger.Error("Failed to send ADS ACK", zap.Error(err))
				return
			}
		}
	}
}

func printSeparator() {
	fmt.Println(strings.Repeat("=", 80))
}

func printSubSeparator() {
	fmt.Println(strings.Repeat("-", 80))
}

func printPolicyResource(index int, runtime *runtimeservice.Runtime) {
	fmt.Printf("\n┌─ Policy Resource #%d\n", index)
	fmt.Printf("│\n")
	fmt.Printf("│  Resource Name: %s\n", runtime.Name)
	fmt.Printf("│\n")

	if runtime.Layer != nil {
		// Unmarshal the complete policy configuration
		policy, err := unmarshalPolicyFromStruct(runtime.Layer)
		if err != nil {
			fmt.Printf("│  ⚠️  Error unmarshaling policy: %v\n", err)
			fmt.Printf("│\n")
			// Fallback to raw display
			printRawLayer(runtime.Layer)
		} else {
			// Display the complete policy structure
			printPolicyDetails(policy)
		}
	}

	// Print raw protobuf as JSON for full details
	fmt.Printf("│\n")
	fmt.Printf("│  🔍 Raw Runtime Resource (JSON):\n")
	jsonData := marshalToJSON(runtime)
	printIndentedJSON(jsonData, "│     ")

	fmt.Printf("│\n")
	fmt.Printf("└─ End of Policy Resource #%d\n", index)
	printSubSeparator()
}

// unmarshalPolicyFromStruct converts structpb.Struct to StoredPolicyConfig
func unmarshalPolicyFromStruct(s *structpb.Struct) (*models.StoredPolicyConfig, error) {
	// Convert struct to JSON
	jsonBytes, err := protojson.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to JSON: %w", err)
	}

	// Unmarshal JSON to StoredPolicyConfig
	var policy models.StoredPolicyConfig
	if err := json.Unmarshal(jsonBytes, &policy); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON to policy: %w", err)
	}

	return &policy, nil
}

// printPolicyDetails displays the complete policy configuration
func printPolicyDetails(policy *models.StoredPolicyConfig) {
	fmt.Printf("│  📋 Policy Information:\n")
	fmt.Printf("│  ├─ ID:      %s\n", policy.ID)
	fmt.Printf("│  └─ Version: %d\n", policy.Version)
	fmt.Printf("│\n")

	// Metadata
	meta := policy.Configuration.Metadata
	fmt.Printf("│  📊 Metadata:\n")
	fmt.Printf("│  ├─ API Name:          %s\n", meta.APIName)
	fmt.Printf("│  ├─ API Version:       %s\n", meta.Version)
	fmt.Printf("│  ├─ Context:           %s\n", meta.Context)
	fmt.Printf("│  ├─ Resource Version:  %d\n", meta.ResourceVersion)
	fmt.Printf("│  ├─ Created At:        %s\n", time.Unix(meta.CreatedAt, 0).Format(time.RFC3339))
	fmt.Printf("│  └─ Updated At:        %s\n", time.Unix(meta.UpdatedAt, 0).Format(time.RFC3339))
	fmt.Printf("│\n")

	// Routes and Policies
	fmt.Printf("│  🛣️  Routes (%d total):\n", len(policy.Configuration.Routes))
	for i, route := range policy.Configuration.Routes {
		fmt.Printf("│\n")
		fmt.Printf("│    Route #%d: %s\n", i+1, route.RouteKey)

		fmt.Printf("│    └─ Policies (%d):\n", len(route.Policies))
		for j, p := range route.Policies {
			fmt.Printf("│       %d. %s (v%s)\n", j+1, p.Name, p.Version)
			if p.ExecutionCondition != nil {
				fmt.Printf("│          Condition: %s\n", *p.ExecutionCondition)
			}
			if len(p.Params) > 0 {
				paramsJSON, _ := json.MarshalIndent(p.Params, "│          ", "  ")
				fmt.Printf("│          Params: %s\n", string(paramsJSON))
			}
		}
	}
}

// printRawLayer is a fallback to display raw struct data
func printRawLayer(s *structpb.Struct) {
	fmt.Printf("│  📊 Raw Layer Data:\n")
	if s.Fields != nil {
		printStructFields(s.Fields, "│     ")
	}
}

func printStructFields(fields map[string]*structpb.Value, indent string) {
	for key, value := range fields {
		switch v := value.Kind.(type) {
		case *structpb.Value_StringValue:
			fmt.Printf("%s%s: \"%s\"\n", indent, key, v.StringValue)
		case *structpb.Value_NumberValue:
			fmt.Printf("%s%s: %.0f\n", indent, key, v.NumberValue)
		case *structpb.Value_BoolValue:
			fmt.Printf("%s%s: %t\n", indent, key, v.BoolValue)
		case *structpb.Value_NullValue:
			fmt.Printf("%s%s: null\n", indent, key)
		case *structpb.Value_StructValue:
			fmt.Printf("%s%s: <nested struct>\n", indent, key)
		case *structpb.Value_ListValue:
			fmt.Printf("%s%s: <list>\n", indent, key)
		}
	}
}

func marshalToJSON(msg proto.Message) string {
	marshaler := protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: false,
	}
	jsonBytes, err := marshaler.Marshal(msg)
	if err != nil {
		return fmt.Sprintf("Error marshaling to JSON: %v", err)
	}
	return string(jsonBytes)
}

func printIndentedJSON(jsonStr string, indent string) {
	// Pretty print JSON with indentation
	var jsonObj interface{}
	if err := json.Unmarshal([]byte(jsonStr), &jsonObj); err == nil {
		prettyJSON, err := json.MarshalIndent(jsonObj, indent, "  ")
		if err == nil {
			lines := strings.Split(string(prettyJSON), "\n")
			for _, line := range lines {
				fmt.Println(indent + line)
			}
			return
		}
	}
	// Fallback: print as-is
	lines := strings.Split(jsonStr, "\n")
	for _, line := range lines {
		fmt.Println(indent + line)
	}
}
