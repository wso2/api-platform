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

package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// FeatureUsage configures reporting of backend feature usage (Moesif API
// events naming a taxonomy feature) to WSO2 Cloud's authenticated Moesif
// collector proxy.
//
// Reporting is off by default. A deployment turns it on with Enabled = true
// (MOESIF_ENABLED in WSO2 Cloud) and configures a collector: CollectorURL and
// the Token client. Neither has a default value, because the collector is a
// WSO2 Cloud service reachable only inside its cluster; enabled without a
// collector, reporting does nothing and says so once at startup.
type FeatureUsage struct {
	// Enabled is the on/off switch (MOESIF_ENABLED in WSO2 Cloud). Off by
	// default.
	Enabled bool `koanf:"enabled"`
	// CollectorURL is the collector proxy's base URL (MOESIF_EVENTS_API_URL in
	// WSO2 Cloud); events are posted to CollectorURL + "/v1/events".
	CollectorURL string `koanf:"collector_url"`
	// Token is the Platform IdP M2M client the proxy's jwt-auth accepts.
	Token FeatureUsageToken `koanf:"token"`
	// QueueSize bounds how many validated events may wait for delivery. A
	// full queue drops the new event instead of blocking the request.
	QueueSize int `koanf:"queue_size"`
	// Workers is how many events are delivered concurrently.
	Workers int `koanf:"workers"`
	// RequestTimeout bounds one request, to the token endpoint or the proxy.
	RequestTimeout time.Duration `koanf:"request_timeout"`
	// MaxAttempts is the total tries for a retryable failure (429, 5xx,
	// timeout, connection error), the first included.
	MaxAttempts int `koanf:"max_attempts"`
	// RetryBackoff is the base wait between attempts, growing linearly.
	RetryBackoff time.Duration `koanf:"retry_backoff"`
	// OverallTimeout bounds the delivery of one event, all attempts included.
	OverallTimeout time.Duration `koanf:"overall_timeout"`
}

// FeatureUsageToken is the OAuth 2.0 client-credentials client used to obtain
// the collector proxy's bearer token.
type FeatureUsageToken struct {
	// TokenURL is the Platform IdP token endpoint (PLATFORM_IDP_TOKEN_URL).
	TokenURL string `koanf:"token_url"`
	// ClientID and ClientSecret identify the M2M client. Supply the secret
	// with a {{ file }} or {{ env }} token; never commit it.
	ClientID     string `koanf:"client_id"`
	ClientSecret string `koanf:"client_secret"`
}

const (
	maxFeatureUsageQueueSize   = 10000
	maxFeatureUsageWorkers     = 16
	maxFeatureUsageMaxAttempts = 5
)

func defaultFeatureUsage() FeatureUsage {
	return FeatureUsage{
		Enabled:        false,
		QueueSize:      256,
		Workers:        2,
		RequestTimeout: 5 * time.Second,
		MaxAttempts:    3,
		RetryBackoff:   time.Second,
		OverallTimeout: 30 * time.Second,
	}
}

// Configured reports whether any collector setting is present. An enabled
// section with none present reports nothing; one with only some present is a
// configuration error, never a silent no-op.
func (c *FeatureUsage) Configured() bool {
	return strings.TrimSpace(c.CollectorURL) != "" || strings.TrimSpace(c.Token.TokenURL) != "" ||
		strings.TrimSpace(c.Token.ClientID) != "" || strings.TrimSpace(c.Token.ClientSecret) != ""
}

// ValidateFeatureUsage applies the config loader's feature-usage checks to a
// config built in code, such as one passed to the platform package's
// WithConfig, which never goes through the loader.
func ValidateFeatureUsage(c *FeatureUsage) error {
	return validateFeatureUsageConfig(c)
}

// validateFeatureUsageConfig checks the settings only when reporting is
// enabled and a collector is being configured; a disabled or unconfigured
// section may be left at its empty defaults.
func validateFeatureUsageConfig(c *FeatureUsage) error {
	if !c.Enabled || !c.Configured() {
		return nil
	}
	if err := validateFeatureUsageURL("feature_usage.collector_url", c.CollectorURL); err != nil {
		return err
	}
	if err := validateFeatureUsageURL("feature_usage.token.token_url", c.Token.TokenURL); err != nil {
		return err
	}
	if strings.TrimSpace(c.Token.ClientID) == "" || strings.TrimSpace(c.Token.ClientSecret) == "" {
		return fmt.Errorf("feature_usage.token.client_id and feature_usage.token.client_secret are required to report feature usage")
	}
	if c.QueueSize < 1 || c.QueueSize > maxFeatureUsageQueueSize {
		return fmt.Errorf("feature_usage.queue_size must be between 1 and %d (got %d)", maxFeatureUsageQueueSize, c.QueueSize)
	}
	if c.Workers < 1 || c.Workers > maxFeatureUsageWorkers {
		return fmt.Errorf("feature_usage.workers must be between 1 and %d (got %d)", maxFeatureUsageWorkers, c.Workers)
	}
	if c.RequestTimeout <= 0 {
		return fmt.Errorf("feature_usage.request_timeout must be a positive duration (got %s)", c.RequestTimeout)
	}
	if c.MaxAttempts < 1 || c.MaxAttempts > maxFeatureUsageMaxAttempts {
		return fmt.Errorf("feature_usage.max_attempts must be between 1 and %d (got %d)", maxFeatureUsageMaxAttempts, c.MaxAttempts)
	}
	if c.RetryBackoff < 0 {
		return fmt.Errorf("feature_usage.retry_backoff must not be negative (got %s)", c.RetryBackoff)
	}
	if c.OverallTimeout < c.RequestTimeout {
		return fmt.Errorf("feature_usage.overall_timeout (%s) must be at least feature_usage.request_timeout (%s)",
			c.OverallTimeout, c.RequestTimeout)
	}
	return nil
}

// validateFeatureUsageURL accepts an absolute http(s) URL with a host and
// nothing that could carry a credential or be misread: no userinfo, query or
// fragment. Plain http is allowed because the collector proxy and the IdP
// token endpoint are in-cluster services in WSO2 Cloud.
func validateFeatureUsageURL(key, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("%s is required to report feature usage", key)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%s is not a valid URL", key)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", key)
	}
	if u.Host == "" {
		return fmt.Errorf("%s must include a host", key)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("%s must not contain user info, a query or a fragment", key)
	}
	return nil
}
