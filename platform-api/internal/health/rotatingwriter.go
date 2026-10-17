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

package health

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// logFileMode is the mode of the live health log and every rotated backup. It is
	// established at creation, so the file is never readable by others, even briefly.
	logFileMode os.FileMode = 0o600
	// logDirMode is the mode of the containing directory when it has to be created.
	logDirMode os.FileMode = 0o700
	// bytesPerMiB converts the configured size threshold into bytes.
	bytesPerMiB int64 = 1 << 20
	// reopenCheckInterval bounds how often the writer checks whether the live file was
	// removed or replaced underneath it, so the check costs at most one stat per second.
	reopenCheckInterval = time.Second
)

// RotatingWriter appends to a health log file and rotates it by size. Rotation renames
// the live file to <path>.1, shifting older backups up to <path>.N and dropping
// anything beyond N, then opens a fresh file, so worst-case disk use is
// maxBytes × (N + 1). Rename-then-recreate is logrotate's "create" mode, which tailing
// log shippers follow by inode.
//
// A write never fails from the caller's point of view: health logging must not affect
// the health answer, so a line that cannot be written is dropped and counted instead.
type RotatingWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	maxFiles int
	now      func() time.Time

	f         *os.File
	size      int64
	lastCheck time.Time
	closed    bool
	dropped   int64
}

// OpenRotatingWriter opens (creating if needed) the health log file at path, which must
// already be cleaned and absolute. maxSizeMB and maxFiles must both be at least 1.
func OpenRotatingWriter(path string, maxSizeMB, maxFiles int) (*RotatingWriter, error) {
	if maxSizeMB < 1 || maxFiles < 1 {
		return nil, fmt.Errorf("health log rotation needs a size of at least 1 MiB and at least 1 backup")
	}
	w := &RotatingWriter{
		path:     path,
		maxBytes: int64(maxSizeMB) * bytesPerMiB,
		maxFiles: maxFiles,
		now:      time.Now,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// Write appends p as one log line, rotating first if it would push the live file past
// the size threshold. It always reports success; a failed write is dropped and counted.
func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		w.dropped++
		return len(p), nil
	}
	w.reopenIfReplaced()
	if w.f == nil {
		w.dropped++
		return len(p), nil
	}

	// The size > 0 guard keeps a line larger than the whole threshold in a fresh file,
	// rather than rotating on every following write and churning the backups away.
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			w.dropped++
			return len(p), nil
		}
	}

	n, err := w.f.Write(p)
	w.size += int64(n)
	if err != nil {
		w.dropped++
		// Close the descriptor so the next write reopens the path instead of retrying a
		// broken one.
		_ = w.f.Close()
		w.f = nil
	}
	return len(p), nil
}

// Dropped returns how many lines could not be written.
func (w *RotatingWriter) Dropped() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.dropped
}

// Close closes the live file. Later writes are dropped.
func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// open creates the directory and the live file if needed and seeds the tracked size, so
// appending to a file left behind by a previous run is accounted for. The caller must
// hold w.mu, or be the constructor.
func (w *RotatingWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(w.path), logDirMode); err != nil {
		return fmt.Errorf("cannot create directory for %q: %w", w.path, err)
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, logFileMode)
	if err != nil {
		return fmt.Errorf("cannot open %q for append: %w", w.path, err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("cannot stat %q: %w", w.path, err)
	}
	w.f = f
	w.size = info.Size()
	w.lastCheck = w.now()
	return nil
}

// reopenIfReplaced reopens the path when the live file was removed or replaced by
// someone else (an operator, or an external rotation), or when an earlier failure left
// no file open. It runs at most once per reopenCheckInterval. The caller must hold w.mu.
func (w *RotatingWriter) reopenIfReplaced() {
	now := w.now()
	if now.Sub(w.lastCheck) < reopenCheckInterval {
		return
	}
	w.lastCheck = now

	if w.f != nil {
		current, err := os.Stat(w.path)
		if err == nil {
			if open, err := w.f.Stat(); err == nil && os.SameFile(current, open) {
				return
			}
		}
		_ = w.f.Close()
		w.f = nil
	}
	if err := w.open(); err != nil {
		w.f = nil
	}
}

// rotate shifts the backups up by one, moves the live file to <path>.1 and opens a fresh
// live file. The caller must hold w.mu.
func (w *RotatingWriter) rotate() error {
	if err := w.f.Close(); err != nil {
		w.f = nil
		return err
	}
	w.f = nil

	if err := removeIfExists(w.backupPath(w.maxFiles)); err != nil {
		return err
	}
	for i := w.maxFiles - 1; i >= 1; i-- {
		if err := renameIfExists(w.backupPath(i), w.backupPath(i+1)); err != nil {
			return err
		}
	}
	if err := renameIfExists(w.path, w.backupPath(1)); err != nil {
		return err
	}
	return w.open()
}

func (w *RotatingWriter) backupPath(i int) string {
	return fmt.Sprintf("%s.%d", w.path, i)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func renameIfExists(from, to string) error {
	if err := os.Rename(from, to); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
