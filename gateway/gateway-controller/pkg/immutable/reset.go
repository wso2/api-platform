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

package immutable

import (
	"fmt"
	"log/slog"
	"os"
)

// ResetSQLiteFiles deletes the SQLite database file and its associated WAL/SHM/journal
// files at the given path. Missing files are treated as success (idempotent).
// Returns an error if any file exists but cannot be removed.
func ResetSQLiteFiles(path string, log *slog.Logger) error {
	suffixes := []string{"", "-wal", "-shm", "-journal"}
	for _, suffix := range suffixes {
		target := path + suffix
		if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %q: %w", target, err)
		}
		if suffix == "" {
			log.Debug("Removed SQLite file for immutable mode reset", slog.String("file", target))
		}
	}
	return nil
}
