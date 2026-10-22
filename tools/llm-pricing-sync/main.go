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

// Command llm-pricing-sync refreshes the Azure entries of the gateway's LLM
// pricing file from the Azure Retail Prices API.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// errOutOfDate reports, in -check mode, that a sync would change a file.
var errOutOfDate = errors.New("pricing files are out of date; run llm-pricing-sync to refresh them")

// stringList is a repeatable string flag.
type stringList []string

func (l *stringList) String() string { return strings.Join(*l, ",") }

func (l *stringList) Set(v string) error {
	*l = append(*l, v)
	return nil
}

func main() {
	err := run(context.Background(), os.Args[1:], os.Stdout)
	switch {
	case errors.Is(err, errOutOfDate):
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	case err != nil:
		fmt.Fprintln(os.Stderr, "llm-pricing-sync:", err)
		os.Exit(2)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	fs := flag.NewFlagSet("llm-pricing-sync", flag.ContinueOnError)
	fs.SetOutput(out)
	var files, meterFiles stringList
	fs.Var(&files, "file", "pricing file to refresh; repeat for each copy")
	fs.Var(&meterFiles, "meters", "read meters from a saved Retail Prices API response instead of calling the API; repeatable")
	check := fs.Bool("check", false, "report whether the files are out of date, without writing them")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("at least one -file is required")
	}

	meters, err := loadMeters(ctx, meterFiles)
	if err != nil {
		return err
	}
	p, skipped, err := buildPrices(meters)
	if err != nil {
		return fmt.Errorf("building prices: %w", err)
	}
	for _, key := range skipped {
		fmt.Fprintf(out, "skipped %s: its meters give no input or no output rate\n", key)
	}

	changed := false
	for _, path := range files {
		report, err := syncFile(path, p, *check)
		if err != nil {
			return err
		}
		printReport(out, path, report)
		changed = changed || report.changed()
	}
	if *check && changed {
		return errOutOfDate
	}
	return nil
}

func loadMeters(ctx context.Context, meterFiles []string) ([]meter, error) {
	if len(meterFiles) > 0 {
		return readMeters(meterFiles)
	}
	return fetchMeters(ctx, &http.Client{Timeout: 60 * time.Second}, retailPricesURL, meterRegions())
}

// syncFile applies p to one pricing file, writing it unless dryRun is set.
func syncFile(path string, p prices, dryRun bool) (syncReport, error) {
	info, err := os.Stat(path)
	if err != nil {
		return syncReport{}, fmt.Errorf("reading %s: %w", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return syncReport{}, fmt.Errorf("reading %s: %w", path, err)
	}
	f, err := parsePricingFile(data)
	if err != nil {
		return syncReport{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	report, err := applyPrices(f, p)
	if err != nil {
		return syncReport{}, fmt.Errorf("updating %s: %w", path, err)
	}
	if dryRun || !report.changed() {
		return report, nil
	}
	encoded, err := f.bytes()
	if err != nil {
		return syncReport{}, fmt.Errorf("encoding %s: %w", path, err)
	}
	if err := os.WriteFile(path, encoded, info.Mode().Perm()); err != nil {
		return syncReport{}, fmt.Errorf("writing %s: %w", path, err)
	}
	return report, nil
}

func printReport(out io.Writer, path string, r syncReport) {
	fmt.Fprintf(out, "%s: %d added, %d updated, %d removed\n", path, len(r.added), len(r.updated), len(r.removed))
	for _, group := range []struct {
		mark string
		keys []string
	}{{"+", r.added}, {"~", r.updated}, {"-", r.removed}} {
		for _, key := range group.keys {
			fmt.Fprintf(out, "  %s %s\n", group.mark, key)
		}
	}
}
