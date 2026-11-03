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

package moesif

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"time"
)

// The event fields that carry a call's billing customer and subscription, and its time.
const (
	customerField     = "metadata.billingCustomerId.raw"
	subscriptionField = "metadata.billingSubscriptionId.raw"
	timeField         = "request.time"
)

// countIDs are the customer and subscription a count is for: values in a search, Moesif's
// templates in a billing meter.
type countIDs struct {
	customer     string
	subscription string
}

// countSignature is what a query counts apart from whose calls: its other clauses and what it adds
// up. A usage search counts what a billing meter counts when their signatures are equal.
func countSignature(query, sum map[string]any) (string, countIDs, error) {
	var ids countIDs
	boolQuery, _ := query["bool"].(map[string]any)
	must, _ := boolQuery["must"].([]any)
	rest := make([]any, 0, len(must))
	for _, clause := range must {
		fields, _ := clause.(map[string]any)
		terms, _ := fields["terms"].(map[string]any)
		values, isTerms := terms[customerField].([]any)
		target := &ids.customer
		if !isTerms {
			values, isTerms = terms[subscriptionField].([]any)
			target = &ids.subscription
		}
		if !isTerms || len(terms) != 1 {
			rest = append(rest, clause)
			continue
		}
		if len(values) != 1 || *target != "" {
			return "", ids, errors.New("a count names one customer and one subscription")
		}
		value, _ := values[0].(string)
		if value == "" {
			return "", ids, errors.New("a count names one customer and one subscription")
		}
		*target = value
	}
	if ids.customer == "" || ids.subscription == "" {
		return "", ids, errors.New("a count must name the customer and the subscription")
	}
	if _, ok := sum["field"].(string); !ok {
		return "", ids, errors.New("a count must add up a field")
	}
	signature, err := json.Marshal(map[string]any{"must": rest, "must_not": boolQuery["must_not"], "sum": sum})
	return string(signature), ids, err
}

type usageRecord struct {
	Subscription string    `json:"subscription"`
	Customer     string    `json:"customer,omitempty"`
	Meter        string    `json:"meter"`
	Time         time.Time `json:"time"`
	Value        int64     `json:"value"`
	signature    string
}

// The bucket sizes of a usage series.
var intervals = map[string]bool{"1h": true, "1d": true, "1w": true}

// bucketStart is the start of the hour, day or week (from Monday) a time falls in.
func bucketStart(t time.Time, interval string, location *time.Location) time.Time {
	local := t.In(location)
	if interval == "1h" {
		return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, location)
	}
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	if interval == "1w" {
		day = day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
	}
	return day
}

func nextBucket(start time.Time, interval string) time.Time {
	switch interval {
	case "1h":
		return start.Add(time.Hour)
	case "1w":
		return start.AddDate(0, 0, 7)
	default:
		return start.AddDate(0, 0, 1)
	}
}

type bucket struct {
	count int64
	value int64
}

// searchEvents answers a usage search: the counted calls of one subscription between from and to,
// summed per hour, day or week. Buckets run from the first to the last that has calls.
func (s *Service) searchEvents(_ string, app *application, w http.ResponseWriter, r *http.Request, body map[string]any) {
	from, errFrom := time.Parse(time.RFC3339Nano, r.URL.Query().Get("from"))
	to, errTo := time.Parse(time.RFC3339Nano, r.URL.Query().Get("to"))
	if errFrom != nil || errTo != nil || to.Before(from) {
		writeError(w, http.StatusBadRequest, "from and to must be ISO 8601 times, from first.")
		return
	}
	series := object(object(body, "aggs"), "series")
	histogram := object(series, "date_histogram")
	interval := text(histogram, "calendar_interval")
	if text(histogram, "field") != timeField || !intervals[interval] {
		writeError(w, http.StatusBadRequest, "aggs.series must be a date_histogram of request.time by 1h, 1d or 1w.")
		return
	}
	location, err := time.LoadLocation(text(histogram, "time_zone"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "time_zone is not a known time zone.")
		return
	}
	signature, ids, err := countSignature(object(body, "query"), object(object(object(series, "aggs"), "usage"), "sum"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error()+".")
		return
	}
	app.mu.Lock()
	buckets := map[time.Time]*bucket{}
	var total int64
	for _, rec := range app.records {
		if rec.signature != signature || rec.Subscription != ids.subscription ||
			(rec.Customer != "" && rec.Customer != ids.customer) || rec.Time.Before(from) || rec.Time.After(to) {
			continue
		}
		start := bucketStart(rec.Time, interval, location)
		if buckets[start] == nil {
			buckets[start] = &bucket{}
		}
		buckets[start].count++
		buckets[start].value += rec.Value
		total++
	}
	app.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"hits":         map[string]any{"total": total},
		"aggregations": map[string]any{"series": map[string]any{"buckets": seriesBuckets(buckets, interval)}},
	})
}

// seriesBuckets lists the buckets in time order, with empty ones between those that have calls.
func seriesBuckets(buckets map[time.Time]*bucket, interval string) []map[string]any {
	starts := make([]time.Time, 0, len(buckets))
	for start := range buckets {
		starts = append(starts, start)
	}
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	out := []map[string]any{}
	if len(starts) == 0 {
		return out
	}
	for start := starts[0]; !start.After(starts[len(starts)-1]); start = nextBucket(start, interval) {
		b := buckets[start]
		if b == nil {
			b = &bucket{}
		}
		out = append(out, map[string]any{
			"key": start.UnixMilli(), "key_as_string": start.Format(time.RFC3339), "doc_count": b.count,
			"usage": map[string]any{"value": b.value},
		})
	}
	return out
}
