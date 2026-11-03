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

package audit

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wso2/api-platform/platform-api/config"
)

// fileSink appends each audit line to a file, rotating at a size ceiling.
//
// Rotation keeps one backup: the live file is renamed to <path>.1, replacing any
// earlier backup, and a new file is opened, so at most 2 x max_size_mb is on disk.
// This is logrotate's "create" mode, which tailers that track inodes (Fluent Bit,
// Filebeat) follow. The sink must be the file's only writer.
type fileSink struct {
	mu sync.Mutex
	// f is nil after Close, or after a rotation could not open a new file.
	f      *os.File
	closed bool
	path   string
	// size is the live file's length, seeded at open so an append to an
	// existing file counts, then advanced by each write.
	size int64
	// maxBytes is the rotation threshold. 0 disables rotation.
	maxBytes int64
	logger   *slog.Logger
	throttle errThrottle
	// buf is reused across writes to append the newline, so each line is one Write.
	buf []byte
}

func newFileSink(cfg config.AuditFile, logger *slog.Logger) (*fileSink, error) {
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
	if err := s.checkPermissions(); err != nil {
		_ = s.f.Close()
		return nil, err
	}
	return s, nil
}

// checkPermissions refuses an existing file that group or others can read: open
// sets the 0600 and 0700 modes only when it creates them. A permissive directory
// only warns, since a 0600 file is unreadable whatever its directory's mode.
func (s *fileSink) checkPermissions() error {
	info, err := s.f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat %q: %w", s.path, err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Errorf("%q has permissions %#o, which allow group/other access; "+
			"fix it with `chmod 600 %s` (or remove the file) and restart", s.path, perm, s.path)
	}
	dir := filepath.Dir(s.path)
	if di, err := os.Stat(dir); err == nil && di.Mode().Perm()&0o077 != 0 {
		s.logger.Warn("Audit log directory is group/other accessible; the log file itself is 0600",
			"dir", dir, "mode", fmt.Sprintf("%#o", di.Mode().Perm()))
	}
	return nil
}

func (s *fileSink) Name() string { return config.AuditOutputFile }

// Write appends the line, rotating first if it would push the file past the size
// ceiling. A failure drops the line and logs.
func (s *fileSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	if s.f == nil {
		// A failed rotation left no file open. Retry on every write, so writing
		// resumes once the directory is usable again.
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
			// Typically a read-only directory blocking the rename: keep writing
			// unrotated rather than dropping records.
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

// rotate renames the live file to <path>.1 and opens a new one. Callers hold s.mu.
// If the rename fails, the original file is reopened so writing continues.
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

// open opens the live file for append, creating it 0600 and its directory 0700
// if missing, and seeds the size from it. Callers hold s.mu, or own s.
func (s *fileSink) open() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(s.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	s.f = f
	s.size = info.Size()
	return nil
}

// Close closes the live file. Writes are unbuffered, so there is nothing to flush.
func (s *fileSink) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.f == nil {
		return nil
	}
	f := s.f
	s.f = nil
	return f.Close()
}
