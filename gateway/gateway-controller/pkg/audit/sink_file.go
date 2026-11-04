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

package audit

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/wso2/api-platform/gateway/gateway-controller/pkg/config"
)

// fileSink rotates by renaming the live file to <path>.1, replacing any
// earlier backup, so at most 2 x max_size_mb is on disk. It must be the only writer.
type fileSink struct {
	mu sync.Mutex
	// f is nil after Close, or after a rotation could not open a new file.
	f        *os.File
	closed   bool
	path     string
	size     int64
	maxBytes int64
	logger   *slog.Logger
	throttle errThrottle
	buf      []byte
}

func newFileSink(cfg config.AuditFileConfig, logger *slog.Logger) (*fileSink, error) {
	path, err := config.ResolveAuditFilePath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if cfg.MaxSizeMB < 0 {
		return nil, fmt.Errorf("max_size_mb must be >= 0, got %d", cfg.MaxSizeMB)
	}
	s := &fileSink{path: path, maxBytes: int64(cfg.MaxSizeMB) << 20, logger: logger}
	if err := s.open(); err != nil {
		return nil, fmt.Errorf("cannot open %q for append: %w", path, err)
	}
	dir := filepath.Dir(path)
	if di, err := os.Stat(dir); err == nil && di.Mode().Perm()&0o077 != 0 {
		logger.Warn("Audit log directory is group/other accessible; the log file itself is 0600",
			"dir", dir, "mode", fmt.Sprintf("%#o", di.Mode().Perm()))
	}
	return s, nil
}

func (s *fileSink) Name() string { return config.AuditOutputFile }

func (s *fileSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	if s.f == nil {
		// A failed rotation left no file open; retry so writing resumes.
		if err := s.open(); err != nil {
			s.throttle.logError(s.logger, "Audit log file is not open; dropping the record", s.Name(), err)
			return
		}
	}

	// The s.size > 0 guard lets a line larger than the whole ceiling land in a
	// fresh file and stay there, instead of rotating on every later write.
	if s.maxBytes > 0 && s.size > 0 && s.size+int64(len(line))+1 > s.maxBytes {
		if err := s.rotate(); err != nil {
			if s.f == nil {
				s.throttle.logError(s.logger, "Failed to rotate the audit log file; dropping the record", s.Name(), err)
				return
			}
			// Usually a read-only directory blocking the rename: keep writing unrotated.
			s.throttle.logError(s.logger, "Failed to rotate the audit log file; writing to it unrotated, "+
				"so it may exceed max_size_mb", s.Name(), err)
		}
	}

	s.buf = append(append(s.buf[:0], line...), '\n')
	n, err := s.f.Write(s.buf)
	// Count a short write too, or the size would drift below the real length.
	s.size += int64(n)
	if err != nil {
		s.throttle.logError(s.logger, "Failed to write audit record to the file; dropping it", s.Name(), err)
	}
}

// rotate reopens the original file if the rename fails, so writing continues.
func (s *fileSink) rotate() error {
	closeErr := s.f.Close()
	s.f = nil

	renameErr := os.Rename(s.path, s.path+".1")

	if err := s.open(); err != nil {
		return fmt.Errorf("reopening after rotation: %w", err)
	}
	if renameErr != nil {
		return fmt.Errorf("renaming to %s: %w", s.path+".1", renameErr)
	}
	if closeErr != nil {
		return fmt.Errorf("closing live file before rotation: %w", closeErr)
	}
	return nil
}

// open is used at startup, after rotation and on recovery. It accepts only a
// regular file: a symlink would redirect the records and a FIFO would block the
// open. O_CREATE sets 0600 only when it creates the file, so an existing file
// that group or others can read is refused.
func (s *fileSink) open() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(s.path); err == nil && !info.Mode().IsRegular() {
		return fmt.Errorf("%q is not a regular file", s.path)
	}
	// O_NONBLOCK: a FIFO swapped in after the Lstat fails the open instead of
	// blocking it; it has no effect on a regular file.
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND|syscall.O_NONBLOCK, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	// Catches a symlink or FIFO swapped in between the Lstat and the open.
	if link, err := os.Lstat(s.path); err != nil || !os.SameFile(info, link) || !info.Mode().IsRegular() {
		_ = f.Close()
		return fmt.Errorf("%q is not a regular file", s.path)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		_ = f.Close()
		return fmt.Errorf("%q has permissions %#o, which allow group/other access; "+
			"fix it with `chmod 600 %s` (or remove the file) and restart", s.path, perm, s.path)
	}
	s.f = f
	s.size = info.Size()
	return nil
}

// Close gives up when ctx ends, so a write stuck on the disk cannot hold shutdown.
func (s *fileSink) Close(ctx context.Context) error {
	done := make(chan error, 1)
	go func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.closed = true
		if s.f == nil {
			done <- nil
			return
		}
		f := s.f
		s.f = nil
		done <- f.Close()
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("audit file output did not close: %w", ctx.Err())
	}
}
