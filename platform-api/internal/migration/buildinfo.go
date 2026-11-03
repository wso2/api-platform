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
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
)

// Build identity. The release build (`make migrate-dist`) stamps these with
// -ldflags "-X …/internal/migration.Version=… -X …Commit=… -X …BuildDate=…" so
// an operator's log proves which build ran. A plain `go build` leaves them
// empty and BuildInfo falls back to the VCS metadata Go embeds on its own.
var (
	Version   = ""
	Commit    = ""
	BuildDate = ""
)

// ErrVersionRequested is returned by ParseFlags when --version was given: the
// caller prints BuildInfo() and exits 0 (mirrors how flag.ErrHelp is handled).
var ErrVersionRequested = errors.New("version requested")

// BuildInfo returns a one-line, log-safe description of this binary:
//
//	v20260929-r1 (commit 0171c336c1ab, built 2026-10-03T14:51:02Z, go1.26.5 linux/amd64)
//
// ldflags-stamped values win; otherwise the embedded vcs.revision / vcs.time
// are used, with "-dirty" appended when the working tree had local changes.
func BuildInfo() string {
	v, c, d := Version, Commit, BuildDate
	dirty := false
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if c == "" {
					c = s.Value
				}
			case "vcs.time":
				if d == "" {
					d = s.Value
				}
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if v == "" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	if v == "" {
		v = "dev"
	}
	if c == "" {
		c = "unknown"
	}
	if len(c) > 12 {
		c = c[:12]
	}
	if dirty && Commit == "" {
		c += "-dirty"
	}
	if d == "" {
		d = "unknown"
	}
	return fmt.Sprintf("%s (commit %s, built %s, %s %s/%s)", v, c, d, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
