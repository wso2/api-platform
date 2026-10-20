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

package health

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

const (
	// fileUnavailableMessage is logged once on stdout when the health log file cannot be
	// opened, after which health events go to stdout only.
	fileUnavailableMessage = "health log file unavailable, logging health events to stdout only"
	// fileExposedMessage is logged on stdout when the health log file or its directory is
	// accessible to group or other users.
	fileExposedMessage = "health log file or directory is accessible to other users"
	// groupOtherBits are the permission bits that make a path accessible beyond its owner.
	groupOtherBits os.FileMode = 0o077
)

// FileLogConfig describes the optional health log file.
type FileLogConfig struct {
	// Path is the cleaned absolute path of the log file. Empty means stdout only.
	Path      string
	MaxSizeMB int
	MaxFiles  int
}

// noopCloser is the closer returned when there is no file to close.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }

// NewLogger returns the logger for health events and a closer for the log file, if any.
// Events always reach stdout; when cfg.Path is set they are also written to a rotating
// file through the handler newFileHandler builds, so file lines use the same format as
// stdout. If the file cannot be opened, one warning is logged and stdout is used alone,
// because health logging must never prevent the service from starting.
func NewLogger(stdout *slog.Logger, newFileHandler func(io.Writer) slog.Handler, cfg FileLogConfig) (*slog.Logger, io.Closer) {
	if cfg.Path == "" {
		return stdout, noopCloser{}
	}

	w, err := OpenRotatingWriter(cfg.Path, cfg.MaxSizeMB, cfg.MaxFiles)
	if err != nil {
		// The error carries the file path, an internal detail, so it is not logged.
		stdout.Warn(fileUnavailableMessage, "reason", "cannot open the health log file")
		return stdout, noopCloser{}
	}

	if mode, exposed := exposedMode(cfg.Path); exposed {
		stdout.Warn(fileExposedMessage, "mode", fmt.Sprintf("%#o", mode))
	}
	return slog.New(slog.NewMultiHandler(stdout.Handler(), newFileHandler(w))), w
}

// exposedMode reports the permission bits of the log file, or of its directory when only
// the directory is accessible beyond its owner.
func exposedMode(path string) (os.FileMode, bool) {
	for _, p := range []string{path, filepath.Dir(path)} {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if mode := info.Mode().Perm(); mode&groupOtherBits != 0 {
			return mode, true
		}
	}
	return 0, false
}
