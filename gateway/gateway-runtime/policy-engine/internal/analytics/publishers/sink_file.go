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

package publishers

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wso2/api-platform/gateway/gateway-runtime/policy-engine/internal/config"
)

const (
	// trafficLogFileMode is the mode of the live log file and every rotated
	// backup. The file holds request and response bodies, so it is owner-only.
	// It is deliberately not configurable: 0600 is the only defensible value for
	// this content, and a knob would only create a way to get it wrong.
	trafficLogFileMode os.FileMode = 0o600
	// trafficLogDirMode is the mode of the containing directory, created if
	// absent. Both modes are established at creation time rather than chmod'd
	// afterwards, which would leave a window where the file is world-readable.
	trafficLogDirMode os.FileMode = 0o700
	// rotatedSuffix is appended to the live path to form the single backup.
	rotatedSuffix = ".1"
	// bytesPerMiB converts the configured max_size_mb into bytes.
	bytesPerMiB int64 = 1 << 20
)

// fileSink appends each traffic-log line to a local file, rotating at a size
// ceiling. It exists so request/response bodies never reach the container's stdout
// — and therefore never reach the node's /var/log/pods files, where every
// node-level log collector (DaemonSet agent, host forwarder) would pick them up
// unredacted.
//
// Rotation keeps exactly one backup: the live file is renamed to <path>.1,
// clobbering any previous backup, and a fresh file is opened. Worst-case on-disk
// total is therefore 2 x maxBytes. That is deliberately simpler than a general
// logging library — this sink is the only writer to the file, so there is no
// concurrent-rotation case to handle, and each additional backup would be another
// copy of PII at rest.
//
// Rename-then-recreate is logrotate's "create" mode, which a tailing reader such as
// Fluent Bit follows correctly via inode tracking.
type fileSink struct {
	mu   sync.Mutex
	f    *os.File
	path string
	// size tracks the live file's length in bytes. It is seeded from Stat at open
	// so an append to a file left behind by a previous run is accounted for, and
	// maintained from write results afterwards — this sink is the only writer, so
	// a per-write Stat would be wasted work.
	size int64
	// maxBytes is the rotation threshold. 0 disables rotation.
	maxBytes int64
	throttle errThrottle
}

// newFileSink opens (creating if needed) the configured traffic-log file.
//
// It returns an error rather than degrading: a file sink that cannot be opened must
// fail startup, never silently leave the operator writing bodies to stdout.
// config.Validate has already performed this same open during startup validation,
// so a failure here means the environment changed underneath us in the interim.
func newFileSink(cfg config.TrafficLogFileConfig) (*fileSink, error) {
	path, err := config.ResolveTrafficLogFilePath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if cfg.MaxSizeMB < 0 {
		return nil, fmt.Errorf("max_size_mb must be >= 0, got %d", cfg.MaxSizeMB)
	}

	if err := os.MkdirAll(filepath.Dir(path), trafficLogDirMode); err != nil {
		return nil, fmt.Errorf("cannot create directory for %q: %w", path, err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, trafficLogFileMode)
	if err != nil {
		return nil, fmt.Errorf("cannot open %q for append: %w", path, err)
	}
	// A pre-existing file or directory kept its old mode: O_CREATE and MkdirAll
	// only apply theirs when they create. Refuse to write bodies into something
	// group- or world-readable.
	if err := config.VerifyTrafficLogPerms(f, path); err != nil {
		_ = f.Close()
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cannot stat %q: %w", path, err)
	}

	s := &fileSink{
		f:        f,
		path:     path,
		size:     info.Size(),
		maxBytes: int64(cfg.MaxSizeMB) * bytesPerMiB,
	}
	slog.Info("Traffic logging file sink ready",
		"path", path, "maxSizeMB", cfg.MaxSizeMB, "existingBytes", info.Size())
	return s, nil
}

// Name returns the sink's identifier.
func (s *fileSink) Name() string { return sinkNameFile }

// Write appends the line, rotating first if it would push the file past the size
// ceiling. A failure drops the line and counts it; it never falls back to stdout
// and never surfaces to the request path.
func (s *fileSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.f == nil { // closed
		mDropped(sinkNameFile, dropReasonWriteFailed, 1)
		return
	}

	// The s.size > 0 guard matters for a line larger than the whole ceiling: it
	// must land in a freshly rotated file and stay there, rather than triggering a
	// rotation on every subsequent write and churning the backup away each time.
	if s.maxBytes > 0 && s.size > 0 && s.size+int64(len(line))+1 > s.maxBytes {
		if err := s.rotate(); err != nil {
			mWriteError(sinkNameFile, errCodeRotate, 1)
			if s.f == nil {
				// No usable handle: the line genuinely cannot be written.
				mDropped(sinkNameFile, dropReasonRotateFailed, 1)
				s.throttle.logError("Failed to rotate traffic-log file; dropping event", sinkNameFile, err)
				return
			}
			// Rotation failed but the file is still writable — typically a
			// read-only parent directory blocking the rename. Keep writing
			// unrotated rather than dropping. The file may now exceed
			// max_size_mb, which is why this is logged as an error and counted.
			s.throttle.logError("Failed to rotate traffic-log file; continuing to write "+
				"unrotated, so the file may exceed max_size_mb", sinkNameFile, err)
		}
	}

	n, err := fmt.Fprintln(s.f, string(line))
	// n counts whatever reached the file even on a short write, so add it before
	// handling the error: otherwise a series of short writes would drift s.size
	// below the real length and defeat the rotation threshold.
	s.size += int64(n)
	if err != nil {
		mDropped(sinkNameFile, dropReasonWriteFailed, 1)
		mWriteError(sinkNameFile, errCodeWrite, 1)
		s.throttle.logError("Failed to write traffic-log event to file; dropping event", sinkNameFile, err)
		return
	}
	mWritten(sinkNameFile, 1)
}

// rotate renames the live file to <path>.1 and reopens a fresh one. Callers must
// hold s.mu.
//
// On failure the sink is left with a usable file wherever possible: if the rename
// fails, the original handle is reopened so writing continues (unrotated) rather
// than the sink going permanently dead and losing every subsequent event.
func (s *fileSink) rotate() error {
	// Retain a Close error rather than returning on it. Returning early would
	// leave s.f pointing at a descriptor Close has already released, so every
	// later write would fail against a dead handle with no attempt to recover.
	// Clearing it and continuing to the reopen below gets the sink working again.
	closeErr := s.f.Close()
	s.f = nil

	renameErr := os.Rename(s.path, s.path+rotatedSuffix)

	// Reopen regardless of the rename result. After a successful rename this
	// creates a new empty file; after a failed one it reattaches to the existing
	// file so the sink keeps working instead of dropping everything from here on.
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, trafficLogFileMode)
	if err != nil {
		return fmt.Errorf("reopening after rotation: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("stat after rotation: %w", err)
	}
	s.f = f
	s.size = info.Size()

	if renameErr != nil {
		return fmt.Errorf("renaming to %s: %w", s.path+rotatedSuffix, renameErr)
	}
	if closeErr != nil {
		// Rotation completed, but the old descriptor did not close cleanly — worth
		// surfacing, and Write treats it as degraded-but-writable since s.f is set.
		return fmt.Errorf("closing live file before rotation: %w", closeErr)
	}
	return nil
}

// Close closes the live file. Writes are unbuffered in userspace, so anything
// already written is in the page cache and needs no flush; Close exists to release
// the descriptor. Safe to call more than once.
func (s *fileSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f == nil {
		return nil
	}
	f := s.f
	s.f = nil
	return f.Close()
}
