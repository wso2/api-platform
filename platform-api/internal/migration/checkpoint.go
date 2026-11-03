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

package migration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Checkpoint is the resumability store. Its job is narrow but critical: any value
// that is NOT deterministic across runs must be recorded so a resumed run reuses
// it. In practice that is exactly the MINTED handles — GenerateHandle's
// random-suffix branch is non-deterministic (§B.2), so a resumed run would
// otherwise mint a different handle and orphan every {{ secret }} ref / child row
// that pointed at the first one. Deterministic uuidv7 values (actors, secrets,
// audit) need no checkpoint — they recompute identically.
//
// It also records which resources have fully completed, so --resume can skip them.
type Checkpoint struct {
	path string
	mu   sync.Mutex

	data checkpointData
}

type checkpointData struct {
	// MintedHandles maps "resource\x00sourceID" -> the minted handle chosen on the
	// first run. sourceID is the v1 natural key (e.g. project uuid).
	MintedHandles map[string]string `json:"mintedHandles"`
	// Completed marks resources whose migration finished cleanly.
	Completed map[string]bool `json:"completed"`
}

// LoadCheckpoint reads the checkpoint file, or returns an empty store if it does
// not yet exist. A resume run passes the same path the first run wrote.
func LoadCheckpoint(path string) (*Checkpoint, error) {
	cp := &Checkpoint{
		path: path,
		data: checkpointData{
			MintedHandles: map[string]string{},
			Completed:     map[string]bool{},
		},
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cp, nil
		}
		return nil, fmt.Errorf("read checkpoint %s: %w", path, err)
	}
	if len(b) == 0 {
		return cp, nil
	}
	if err := json.Unmarshal(b, &cp.data); err != nil {
		return nil, fmt.Errorf("parse checkpoint %s: %w", path, err)
	}
	if cp.data.MintedHandles == nil {
		cp.data.MintedHandles = map[string]string{}
	}
	if cp.data.Completed == nil {
		cp.data.Completed = map[string]bool{}
	}
	return cp, nil
}

func handleKey(resource, sourceID string) string {
	return resource + "\x00" + sourceID
}

// Handle returns a previously minted handle for (resource, sourceID) if one was
// recorded.
func (c *Checkpoint) Handle(resource, sourceID string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	h, ok := c.data.MintedHandles[handleKey(resource, sourceID)]
	return h, ok
}

// PutHandle records a minted handle and persists the store.
func (c *Checkpoint) PutHandle(resource, sourceID, handle string) error {
	c.mu.Lock()
	c.data.MintedHandles[handleKey(resource, sourceID)] = handle
	c.mu.Unlock()
	return c.save()
}

// IsComplete reports whether a resource finished in a prior run.
// Stats reports how much state the store holds: minted handles and resources
// marked complete. The runner logs both at start-up so a leftover file from a
// previous run is visible instead of silently steering this one.
func (c *Checkpoint) Stats() (minted, completed int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.data.MintedHandles), len(c.data.Completed)
}

func (c *Checkpoint) IsComplete(resource string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data.Completed[resource]
}

// MarkComplete records a resource as finished and persists the store.
func (c *Checkpoint) MarkComplete(resource string) error {
	c.mu.Lock()
	c.data.Completed[resource] = true
	c.mu.Unlock()
	return c.save()
}

// save atomically rewrites the checkpoint file (temp + rename) so a crash mid-write
// never corrupts it.
func (c *Checkpoint) save() error {
	if c.path == "" {
		return nil
	}
	c.mu.Lock()
	b, err := json.MarshalIndent(c.data, "", "  ")
	c.mu.Unlock()
	if err != nil {
		return fmt.Errorf("marshal checkpoint: %w", err)
	}
	dir := filepath.Dir(c.path)
	tmp, err := os.CreateTemp(dir, ".migration-checkpoint-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp checkpoint: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return fmt.Errorf("write temp checkpoint: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp checkpoint: %w", err)
	}
	if err := os.Rename(tmpName, c.path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename checkpoint into place: %w", err)
	}
	return nil
}
