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
	"fmt"
	"log/slog"
	"os"
	"text/tabwriter"
)

// printReport writes the final per-resource PASS/WARN/FAIL table to stdout and a
// one-line summary through the logger. Row-count parity and any per-resource notes
// come from each migrator's Verify (§9).
func printReport(log *slog.Logger, reports []*ResourceReport, cfg *Config) {
	fmt.Println()
	fmt.Printf("================ MIGRATION REPORT (%s) ================\n", cfg.Direction)
	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "RESOURCE\tSTATUS\tSRC\tTGT\tNOTES")
	var pass, warn, fail, skip int
	for _, r := range reports {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusWarn:
			warn++
		case StatusFail:
			fail++
		case StatusSkip:
			skip++
		}
		note := ""
		if len(r.Messages) > 0 {
			note = r.Messages[0]
			if len(r.Messages) > 1 {
				note += fmt.Sprintf(" (+%d more)", len(r.Messages)-1)
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\n", r.Name, r.Status, r.SrcCount, r.TgtCount, note)
	}
	tw.Flush()
	fmt.Printf("---------------------------------------------------------\n")
	fmt.Printf("PASS=%d  WARN=%d  FAIL=%d  SKIP=%d  (total=%d)\n", pass, warn, fail, skip, len(reports))
	fmt.Println("=========================================================")

	// Emit the full message list for any non-PASS resource through the logger.
	for _, r := range reports {
		if r.Status == StatusPass || r.Status == StatusSkip {
			continue
		}
		for _, m := range r.Messages {
			log.Warn("report detail", "resource", r.Name, "status", string(r.Status), "message", m)
		}
	}
	if fail > 0 {
		log.Error("migration report: FAILURES present", "fail", fail)
	} else {
		log.Info("migration report complete", "pass", pass, "warn", warn, "skip", skip)
	}
}
