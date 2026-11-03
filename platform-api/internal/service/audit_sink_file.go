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

package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/wso2/api-platform/platform-api/config"
)

// auditFileSink appends each audit line to a file, rotating at a size ceiling.
//
// Rotation keeps one backup: the live file is renamed to <path>.1, replacing any
// earlier backup, and a new file is opened, so at most 2 x max_size_mb is on disk.
// This is logrotate's "create" mode, which tailers that track inodes (Fluent Bit,
// Filebeat) follow. The sink must be the file's only writer.
type auditFileSink struct {
	mu sync.Mutex
	// f is nil after Close, or after a rotation could not open a new file.
	f      *os.File
	closed bool
	path   string
	// size is the live file's length, seeded from Stat at open so an append to an
	// existing file counts, then advanced by each write.
	size int64
	// maxBytes is the rotation threshold. 0 disables rotation.
	maxBytes int64
	logger   *slog.Logger
	throttle auditErrThrottle
	// buf is reused across writes to append the newline, so each line is one Write.
	buf []byte
}

// newAuditFileSink opens (creating if needed) the configured audit log file. It
// returns an error rather than degrading; config validation has already done the
// same open, so a failure here means the environment changed in between.
func newAuditFileSink(cfg config.AuditFile, logger *slog.Logger) (*auditFileSink, error) {
	path, err := config.ResolveAuditFilePath(cfg.Path)
	if err != nil {
		return nil, err
	}
	if cfg.MaxSizeMB < 0 {
		return nil, fmt.Errorf("max_size_mb must be >= 0, got %d", cfg.MaxSizeMB)
	}
	f, err := config.OpenAuditFile(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("cannot stat %q: %w", path, err)
	}
	return &auditFileSink{
		f:        f,
		path:     path,
		size:     info.Size(),
		maxBytes: int64(cfg.MaxSizeMB) << 20,
		logger:   logger,
	}, nil
}

func (s *auditFileSink) Name() string { return config.AuditOutputFile }

// Write appends the line, rotating first if it would push the file past the size
// ceiling. A failure drops the line and logs.
func (s *auditFileSink) Write(line []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	if s.f == nil {
		// A failed rotation left no file open. Retry on every write, so writing
		// resumes once the directory is usable again.
		if err := s.open(); err != nil {
			s.throttle.logError(s.logger, "Audit log file is not open; dropping the record",
				config.AuditOutputFile, err)
			return
		}
	}

	// The s.size > 0 guard lets a line larger than the whole ceiling land in a
	// fresh file and stay there, instead of rotating on every later write.
	if s.maxBytes > 0 && s.size > 0 && s.size+int64(len(line))+1 > s.maxBytes {
		if err := s.rotate(); err != nil {
			if s.f == nil {
				s.throttle.logError(s.logger, "Failed to rotate the audit log file; dropping the record",
					config.AuditOutputFile, err)
				return
			}
			// Typically a read-only directory blocking the rename: keep writing
			// unrotated rather than dropping records.
			s.throttle.logError(s.logger, "Failed to rotate the audit log file; writing to it unrotated, "+
				"so it may exceed max_size_mb", config.AuditOutputFile, err)
		}
	}

	s.buf = append(append(s.buf[:0], line...), '\n')
	n, err := s.f.Write(s.buf)
	// Count a short write too, or the size would drift below the real length.
	s.size += int64(n)
	if err != nil {
		s.throttle.logError(s.logger, "Failed to write audit record to the file; dropping it",
			config.AuditOutputFile, err)
	}
}

// rotate renames the live file to <path>.1 and opens a new one. Callers hold s.mu.
// If the rename fails, the original file is reopened so writing continues.
func (s *auditFileSink) rotate() error {
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

// open opens the live file for append, recreating its directory (0700) if it
// was removed, and seeds the size from it. Callers hold s.mu.
func (s *auditFileSink) open() error {
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
func (s *auditFileSink) Close(context.Context) error {
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
