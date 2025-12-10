package urlguardrail

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
	TextCleanRegex             = "^\"|\"$"
	URLRegex                   = "https?://[^\\s,\"'{}\\[\\]\\\\`*]+"
	DefaultTimeout             = 3000 // milliseconds
)

var (
	textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)
	urlRegexCompiled       = regexp.MustCompile(URLRegex)
)

// URLGuardrailPolicy implements URL validation guardrail
type URLGuardrailPolicy struct{}

// NewPolicy creates a new URLGuardrailPolicy instance
func NewPolicy() policy.Policy {
	return &URLGuardrailPolicy{}
}

// Mode returns the processing mode for this policy
func (p *URLGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest validates URLs in request body
func (p *URLGuardrailPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate parameters
	if err := p.validateParams(requestParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), false, false, []string{}).(policy.RequestAction)
	}

	return p.validatePayload(ctx.Body.Content, requestParams, false).(policy.RequestAction)
}

// OnResponse validates URLs in response body
func (p *URLGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate parameters
	if err := p.validateParams(responseParams); err != nil {
		return p.buildErrorResponse(fmt.Sprintf("parameter validation failed: %v", err), true, false, []string{}).(policy.ResponseAction)
	}

	return p.validatePayload(ctx.ResponseBody.Content, responseParams, true).(policy.ResponseAction)
}

// validateParams validates the actual policy parameters
func (p *URLGuardrailPolicy) validateParams(params map[string]interface{}) error {
	// Validate optional parameters
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		_, ok := jsonPathRaw.(string)
		if !ok {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	}

	if onlyDNSRaw, ok := params["onlyDNS"]; ok {
		_, ok := onlyDNSRaw.(bool)
		if !ok {
			return fmt.Errorf("'onlyDNS' must be a boolean")
		}
	}

	if timeoutRaw, ok := params["timeout"]; ok {
		timeout, ok := timeoutRaw.(float64)
		if !ok {
			if timeoutInt, ok := timeoutRaw.(int); ok {
				timeout = float64(timeoutInt)
			} else if timeoutStr, ok := timeoutRaw.(string); ok {
				var err error
				timeout, err = strconv.ParseFloat(timeoutStr, 64)
				if err != nil {
					return fmt.Errorf("'timeout' must be a number")
				}
			} else {
				return fmt.Errorf("'timeout' must be a number")
			}
		}
		if timeout < 0 {
			return fmt.Errorf("'timeout' cannot be negative")
		}
	}

	if showAssessmentRaw, ok := params["showAssessment"]; ok {
		_, ok := showAssessmentRaw.(bool)
		if !ok {
			return fmt.Errorf("'showAssessment' must be a boolean")
		}
	}

	return nil
}

// validatePayload validates URLs in payload
func (p *URLGuardrailPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool) interface{} {
	jsonPath, _ := params["jsonPath"].(string)
	onlyDNS, _ := params["onlyDNS"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	timeout := DefaultTimeout
	if timeoutRaw, ok := params["timeout"]; ok {
		if timeoutFloat, ok := timeoutRaw.(float64); ok {
			timeout = int(timeoutFloat)
		} else if timeoutInt, ok := timeoutRaw.(int); ok {
			timeout = timeoutInt
		}
	}

	if payload == nil {
		return p.buildErrorResponse("body is empty", isResponse, showAssessment, []string{})
	}

	// Extract value using JSONPath
	extractedValue, err := extractStringValueFromJSONPath(payload, jsonPath)
	if err != nil {
		return p.buildErrorResponse(fmt.Sprintf("error extracting value from JSONPath: %v", err), isResponse, showAssessment, []string{})
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Extract URLs from the value
	urls := urlRegexCompiled.FindAllString(extractedValue, -1)
	invalidURLs := make([]string, 0)

	for _, urlStr := range urls {
		var isValid bool
		if onlyDNS {
			isValid = p.checkDNS(urlStr, timeout)
		} else {
			isValid = p.checkURL(urlStr, timeout)
		}

		if !isValid {
			invalidURLs = append(invalidURLs, urlStr)
		}
	}

	if len(invalidURLs) > 0 {
		return p.buildErrorResponse("violation of url validity detected", isResponse, showAssessment, invalidURLs)
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// checkDNS checks if the URL is resolved via DNS
func (p *URLGuardrailPolicy) checkDNS(target string, timeout int) bool {
	parsedURL, err := url.Parse(target)
	if err != nil {
		return false
	}

	host := parsedURL.Hostname()
	if host == "" {
		return false
	}

	// Create a custom resolver with timeout
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Duration(timeout) * time.Millisecond,
			}
			return d.DialContext(ctx, network, address)
		},
	}

	// Look up IP addresses
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Millisecond)
	defer cancel()

	ips, err := resolver.LookupIP(ctx, "ip", host)
	if err != nil {
		return false
	}

	return len(ips) > 0
}

// checkURL checks if the URL is reachable via HTTP HEAD request
func (p *URLGuardrailPolicy) checkURL(target string, timeout int) bool {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}

	req, err := http.NewRequest("HEAD", target, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "URLValidator/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	statusCode := resp.StatusCode
	return statusCode >= 200 && statusCode < 400
}

// buildErrorResponse builds an error response for both request and response phases
func (p *URLGuardrailPolicy) buildErrorResponse(reason string, isResponse bool, showAssessment bool, invalidURLs []string) interface{} {
	assessment := p.buildAssessmentObject(isResponse, reason, showAssessment, invalidURLs)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "URL_GUARDRAIL",
		"message": assessment,
	}

	bodyBytes, _ := json.Marshal(responseBody)

	if isResponse {
		statusCode := GuardrailErrorCode
		return policy.UpstreamResponseModifications{
			StatusCode: &statusCode,
			Body:       bodyBytes,
			SetHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		}
	}

	return policy.ImmediateResponse{
		StatusCode: GuardrailErrorCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: bodyBytes,
	}
}

// buildAssessmentObject builds the assessment object
func (p *URLGuardrailPolicy) buildAssessmentObject(isResponse bool, reason string, showAssessment bool, invalidURLs []string) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "URLGuardrail",
		"actionReason":         "Violation of url validity detected.",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if showAssessment && len(invalidURLs) > 0 {
		assessmentDetails := map[string]interface{}{
			"message":     "One or more URLs in the payload failed validation.",
			"invalidUrls": invalidURLs,
		}
		assessment["assessments"] = assessmentDetails
	}

	return assessment
}

// extractStringValueFromJSONPath extracts a value from JSON using JSONPath
func extractStringValueFromJSONPath(payload []byte, jsonPath string) (string, error) {
	if jsonPath == "" {
		return string(payload), nil
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(payload, &jsonData); err != nil {
		return "", fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	value, err := extractValueFromJSONPath(jsonData, jsonPath)
	if err != nil {
		return "", err
	}

	// Convert to string
	switch v := value.(type) {
	case string:
		return v, nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// extractValueFromJSONPath extracts a value from a nested JSON structure based on a JSON path
func extractValueFromJSONPath(data map[string]interface{}, jsonPath string) (interface{}, error) {
	keys := strings.Split(jsonPath, ".")
	if len(keys) > 0 && keys[0] == "$" {
		keys = keys[1:]
	}

	return extractRecursive(data, keys)
}

func extractRecursive(current interface{}, keys []string) (interface{}, error) {
	if len(keys) == 0 {
		return current, nil
	}

	key := keys[0]
	remaining := keys[1:]

	// Handle array indexing
	arrayIndexRegex := regexp.MustCompile(`^([a-zA-Z0-9_]+)\[(-?\d+)\]$`)
	if matches := arrayIndexRegex.FindStringSubmatch(key); len(matches) == 3 {
		arrayName := matches[1]
		idxStr := matches[2]
		idx := 0
		fmt.Sscanf(idxStr, "%d", &idx)

		if node, ok := current.(map[string]interface{}); ok {
			if arrVal, exists := node[arrayName]; exists {
				if arr, ok := arrVal.([]interface{}); ok {
					if idx < 0 {
						idx = len(arr) + idx
					}
					if idx < 0 || idx >= len(arr) {
						return nil, fmt.Errorf("array index out of range: %d", idx)
					}
					return extractRecursive(arr[idx], remaining)
				}
				return nil, fmt.Errorf("not an array: %s", arrayName)
			}
			return nil, fmt.Errorf("key not found: %s", arrayName)
		}
		return nil, fmt.Errorf("invalid structure for key: %s", arrayName)
	}

	// Handle wildcard
	if key == "*" {
		var results []interface{}
		switch node := current.(type) {
		case map[string]interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		case []interface{}:
			for _, v := range node {
				res, err := extractRecursive(v, remaining)
				if err == nil {
					results = append(results, res)
				}
			}
		default:
			return nil, fmt.Errorf("wildcard used on non-iterable node")
		}
		return results, nil
	}

	// Regular object key
	if node, ok := current.(map[string]interface{}); ok {
		if val, exists := node[key]; exists {
			return extractRecursive(val, remaining)
		}
		return nil, fmt.Errorf("key not found: %s", key)
	}

	return nil, fmt.Errorf("invalid structure for key: %s", key)
}
