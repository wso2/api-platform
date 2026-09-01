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

package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
)

// Network is one block's private docker network.
//
// Its lifetime is the block's: created before the first container starts, removed
// after the last one stops. Because it is private, alias collisions between blocks are
// impossible by construction rather than avoided by scheduling.
type Network struct {
	name  string
	block string
	inner *testcontainers.DockerNetwork
}

// NewNetwork creates a network for the named block.
//
// The block label is carried in the network name so a leaked network is attributable
// to the block that leaked it — with parallel blocks, an orphan named only by a random
// token tells you nothing.
func NewNetwork(ctx context.Context, block string) (*Network, error) {
	if ctx == nil {
		return nil, fmt.Errorf("runtime: network creation requires a context")
	}
	if strings.TrimSpace(block) == "" {
		return nil, fmt.Errorf("runtime: a network needs a block label")
	}
	nw, err := tcnetwork.New(ctx,
		tcnetwork.WithLabels(map[string]string{
			labelBlock:     block,
			labelFramework: frameworkLabelValue,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("runtime: creating network for block %q: %w", block, err)
	}

	return &Network{name: nw.Name, block: block, inner: nw}, nil
}

// Name is the docker network name, which containers join and which appears in
// diagnostics.
func (n *Network) Name() string {
	if n == nil {
		return ""
	}
	return n.name
}

// Block is the label of the block that owns this network.
func (n *Network) Block() string {
	if n == nil {
		return ""
	}
	return n.block
}

// Remove tears the network down.
//
// Safe to call more than once: teardown runs on both the success and boot-failure
// paths, and a second call must not turn a real failure into a confusing one.
func (n *Network) Remove(ctx context.Context) error {
	if n == nil || n.inner == nil {
		return nil
	}
	inner := n.inner
	if err := inner.Remove(ctx); err != nil {
		return fmt.Errorf("runtime: removing network %q for block %q: %w", n.name, n.block, err)
	}
	n.inner = nil
	return nil
}
