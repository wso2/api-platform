package awsbedrockguardrail

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
	utils "github.com/wso2/api-platform/sdk/utils"
)

const (
	GuardrailErrorCode         = 446
	GuardrailAPIMExceptionCode = 900514
	TextCleanRegex             = "^\"|\"$"
	MetadataKeyPIIEntities     = "awsbedrockguardrail:pii_entities"
)

var textCleanRegexCompiled = regexp.MustCompile(TextCleanRegex)

// AWSBedrockGuardrailPolicy implements AWS Bedrock Guardrail validation
type AWSBedrockGuardrailPolicy struct {
	// Static configuration from initParams
	region             string
	guardrailID        string
	guardrailVersion   string
	awsAccessKeyID     string
	awsSecretAccessKey string
	awsSessionToken    string
	awsRoleARN         string
	awsRoleRegion      string
	awsRoleExternalID  string
}

// NewPolicy creates a new AWSBedrockGuardrailPolicy instance
func NewPolicy(
	metadata policy.PolicyMetadata,
	initParams map[string]interface{},
	params map[string]interface{},
) (policy.Policy, error) {
	// Validate and extract static configuration from initParams
	if err := validateAWSConfigParams(initParams); err != nil {
		return nil, fmt.Errorf("invalid initParams: %w", err)
	}

	policy := &AWSBedrockGuardrailPolicy{
		region:           getStringParam(initParams, "region"),
		guardrailID:      getStringParam(initParams, "guardrailID"),
		guardrailVersion: getStringParam(initParams, "guardrailVersion"),
	}

	// Optional AWS credentials
	if val, ok := initParams["awsAccessKeyID"]; ok {
		policy.awsAccessKeyID = val.(string)
	}
	if val, ok := initParams["awsSecretAccessKey"]; ok {
		policy.awsSecretAccessKey = val.(string)
	}
	if val, ok := initParams["awsSessionToken"]; ok {
		policy.awsSessionToken = val.(string)
	}
	if val, ok := initParams["awsRoleARN"]; ok {
		policy.awsRoleARN = val.(string)
	}
	if val, ok := initParams["awsRoleRegion"]; ok {
		policy.awsRoleRegion = val.(string)
	}
	if val, ok := initParams["awsRoleExternalID"]; ok {
		policy.awsRoleExternalID = val.(string)
	}

	return policy, nil
}

// getStringParam safely extracts a string parameter
func getStringParam(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// validateAWSConfigParams validates AWS configuration parameters (from initParams)
func validateAWSConfigParams(params map[string]interface{}) error {
	// Validate region (required)
	regionRaw, ok := params["region"]
	if !ok {
		return fmt.Errorf("'region' parameter is required")
	}
	region, ok := regionRaw.(string)
	if !ok {
		return fmt.Errorf("'region' must be a string")
	}
	if region == "" {
		return fmt.Errorf("'region' cannot be empty")
	}

	// Validate guardrailID (required)
	guardrailIDRaw, ok := params["guardrailID"]
	if !ok {
		return fmt.Errorf("'guardrailID' parameter is required")
	}
	guardrailID, ok := guardrailIDRaw.(string)
	if !ok {
		return fmt.Errorf("'guardrailID' must be a string")
	}
	if guardrailID == "" {
		return fmt.Errorf("'guardrailID' cannot be empty")
	}

	// Validate guardrailVersion (required)
	guardrailVersionRaw, ok := params["guardrailVersion"]
	if !ok {
		return fmt.Errorf("'guardrailVersion' parameter is required")
	}
	guardrailVersion, ok := guardrailVersionRaw.(string)
	if !ok {
		return fmt.Errorf("'guardrailVersion' must be a string")
	}
	if guardrailVersion == "" {
		return fmt.Errorf("'guardrailVersion' cannot be empty")
	}

	// Validate optional AWS credential parameters
	if awsAccessKeyIDRaw, ok := params["awsAccessKeyID"]; ok {
		awsAccessKeyID, ok := awsAccessKeyIDRaw.(string)
		if !ok {
			return fmt.Errorf("'awsAccessKeyID' must be a string")
		}
		if awsAccessKeyID == "" {
			return fmt.Errorf("'awsAccessKeyID' cannot be empty")
		}
	}

	if awsSecretAccessKeyRaw, ok := params["awsSecretAccessKey"]; ok {
		awsSecretAccessKey, ok := awsSecretAccessKeyRaw.(string)
		if !ok {
			return fmt.Errorf("'awsSecretAccessKey' must be a string")
		}
		if awsSecretAccessKey == "" {
			return fmt.Errorf("'awsSecretAccessKey' cannot be empty")
		}
	}

	if awsSessionTokenRaw, ok := params["awsSessionToken"]; ok {
		_, ok := awsSessionTokenRaw.(string)
		if !ok {
			return fmt.Errorf("'awsSessionToken' must be a string")
		}
	}

	if awsRoleARNRaw, ok := params["awsRoleARN"]; ok {
		awsRoleARN, ok := awsRoleARNRaw.(string)
		if !ok {
			return fmt.Errorf("'awsRoleARN' must be a string")
		}
		if awsRoleARN == "" {
			return fmt.Errorf("'awsRoleARN' cannot be empty")
		}

		// If role ARN is provided, validate role region
		if awsRoleRegionRaw, ok := params["awsRoleRegion"]; ok {
			awsRoleRegion, ok := awsRoleRegionRaw.(string)
			if !ok {
				return fmt.Errorf("'awsRoleRegion' must be a string")
			}
			if awsRoleRegion == "" {
				return fmt.Errorf("'awsRoleRegion' cannot be empty")
			}
		}
	}

	if awsRoleExternalIDRaw, ok := params["awsRoleExternalID"]; ok {
		_, ok := awsRoleExternalIDRaw.(string)
		if !ok {
			return fmt.Errorf("'awsRoleExternalID' must be a string")
		}
	}

	return nil
}

// Mode returns the processing mode for this policy
func (p *AWSBedrockGuardrailPolicy) Mode() policy.ProcessingMode {
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeSkip,
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeSkip,
		ResponseBodyMode:   policy.BodyModeBuffer,
	}
}

// OnRequest validates request body using AWS Bedrock Guardrail
func (p *AWSBedrockGuardrailPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	var requestParams map[string]interface{}
	if reqParams, ok := params["request"].(map[string]interface{}); ok {
		requestParams = reqParams
	} else {
		return policy.UpstreamRequestModifications{}
	}

	// Validate request-specific parameters
	if err := p.validateRequestResponseParams(requestParams); err != nil {
		return p.buildErrorResponse("Parameter validation failed", err, false, false, nil).(policy.RequestAction)
	}

	var content []byte
	if ctx.Body != nil {
		content = ctx.Body.Content
	}
	return p.validatePayload(content, requestParams, false, ctx.Metadata).(policy.RequestAction)
}

// OnResponse validates response body using AWS Bedrock Guardrail
func (p *AWSBedrockGuardrailPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	var responseParams map[string]interface{}
	if respParams, ok := params["response"].(map[string]interface{}); ok {
		responseParams = respParams
	} else {
		return policy.UpstreamResponseModifications{}
	}

	// Validate response-specific parameters
	if err := p.validateRequestResponseParams(responseParams); err != nil {
		return p.buildErrorResponse("Parameter validation failed", err, true, false, nil).(policy.ResponseAction)
	}

	var content []byte
	if ctx.ResponseBody != nil {
		content = ctx.ResponseBody.Content
	}
	return p.validatePayload(content, responseParams, true, ctx.Metadata).(policy.ResponseAction)
}

// validateRequestResponseParams validates request/response specific parameters
func (p *AWSBedrockGuardrailPolicy) validateRequestResponseParams(params map[string]interface{}) error {
	// Validate optional parameters
	if jsonPathRaw, ok := params["jsonPath"]; ok {
		_, ok := jsonPathRaw.(string)
		if !ok {
			return fmt.Errorf("'jsonPath' must be a string")
		}
	}

	if redactPIIRaw, ok := params["redactPII"]; ok {
		_, ok := redactPIIRaw.(bool)
		if !ok {
			return fmt.Errorf("'redactPII' must be a boolean")
		}
	}

	if passthroughOnErrorRaw, ok := params["passthroughOnError"]; ok {
		_, ok := passthroughOnErrorRaw.(bool)
		if !ok {
			return fmt.Errorf("'passthroughOnError' must be a boolean")
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

// validatePayload validates payload against AWS Bedrock Guardrail
func (p *AWSBedrockGuardrailPolicy) validatePayload(payload []byte, params map[string]interface{}, isResponse bool, metadata map[string]interface{}) interface{} {
	jsonPath, _ := params["jsonPath"].(string)
	redactPII, _ := params["redactPII"].(bool)
	passthroughOnError, _ := params["passthroughOnError"].(bool)
	showAssessment, _ := params["showAssessment"].(bool)

	// Transform response if redactPII is disabled and PIIs identified in request
	if !redactPII && isResponse {
		if maskedPII, exists := metadata[MetadataKeyPIIEntities]; exists {
			if maskedPIIMap, ok := maskedPII.(map[string]string); ok {
				// Restore PII in response
				restoredContent := p.restorePIIInResponse(string(payload), maskedPIIMap)
				if restoredContent != string(payload) {
					return policy.UpstreamResponseModifications{
						Body: []byte(restoredContent),
					}
				}
			}
		}
	}

	if payload == nil {
		if isResponse {
			return policy.UpstreamResponseModifications{}
		}
		return policy.UpstreamRequestModifications{}
	}

	// Extract value using JSONPath
	extractedValue, err := utils.ExtractStringValueFromJsonpath(payload, jsonPath)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse("Error extracting value from JSONPath", err, isResponse, showAssessment, nil)
	}

	// Clean and trim
	extractedValue = textCleanRegexCompiled.ReplaceAllString(extractedValue, "")
	extractedValue = strings.TrimSpace(extractedValue)

	// Create AWS config
	awsCfg, err := p.loadAWSConfig(context.Background(), p.region)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse("Error loading AWS config", err, isResponse, showAssessment, nil)
	}

	// Call AWS Bedrock Guardrail
	output, err := p.applyBedrockGuardrail(context.Background(), awsCfg, p.guardrailID, p.guardrailVersion, extractedValue)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse("Error calling AWS Bedrock Guardrail", err, isResponse, showAssessment, nil)
	}

	// Evaluate guardrail response
	var outputInterface interface{} = output
	violation, modifiedContent, err := p.evaluateGuardrailResponse(outputInterface, extractedValue, redactPII, !isResponse, metadata)
	if err != nil {
		if passthroughOnError {
			if isResponse {
				return policy.UpstreamResponseModifications{}
			}
			return policy.UpstreamRequestModifications{}
		}
		return p.buildErrorResponse("Error evaluating guardrail response", err, isResponse, showAssessment, output)
	}

	if violation {
		return p.buildErrorResponse("Violation of AWS Bedrock Guardrails detected", nil, isResponse, showAssessment, output)
	}

	// If content was modified, update the payload
	if modifiedContent != "" && modifiedContent != extractedValue {
		modifiedPayload := p.updatePayloadWithMaskedContent(payload, extractedValue, modifiedContent, jsonPath)
		if isResponse {
			return policy.UpstreamResponseModifications{
				Body: modifiedPayload,
			}
		}
		return policy.UpstreamRequestModifications{
			Body: modifiedPayload,
		}
	}

	if isResponse {
		return policy.UpstreamResponseModifications{}
	}
	return policy.UpstreamRequestModifications{}
}

// loadAWSConfig creates AWS configuration with custom credentials and role assumption
func (p *AWSBedrockGuardrailPolicy) loadAWSConfig(ctx context.Context, region string) (aws.Config, error) {
	// Use AWS credentials from policy instance (initParams)
	accessKeyID := p.awsAccessKeyID
	secretAccessKey := p.awsSecretAccessKey
	sessionToken := p.awsSessionToken
	roleARN := p.awsRoleARN
	roleRegion := p.awsRoleRegion
	roleExternalID := p.awsRoleExternalID

	// Check if role-based authentication should be used
	if roleARN != "" && roleRegion != "" {
		return p.loadAWSConfigWithAssumeRole(ctx, accessKeyID, secretAccessKey, sessionToken, roleARN, roleRegion, roleExternalID, region)
	} else if accessKeyID != "" && secretAccessKey != "" {
		return p.loadAWSConfigWithStaticCredentials(ctx, accessKeyID, secretAccessKey, sessionToken, region)
	} else {
		// Use default credential chain
		return config.LoadDefaultConfig(ctx, config.WithRegion(region))
	}
}

// loadAWSConfigWithStaticCredentials creates AWS config with static credentials
func (p *AWSBedrockGuardrailPolicy) loadAWSConfigWithStaticCredentials(ctx context.Context, accessKeyID, secretAccessKey, sessionToken, region string) (aws.Config, error) {
	var credsProvider aws.CredentialsProvider
	if sessionToken != "" {
		credsProvider = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)
	} else {
		credsProvider = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(credsProvider),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config with static credentials: %w", err)
	}

	return cfg, nil
}

// loadAWSConfigWithAssumeRole creates AWS config with role assumption
func (p *AWSBedrockGuardrailPolicy) loadAWSConfigWithAssumeRole(ctx context.Context, accessKeyID, secretAccessKey, sessionToken, roleARN, roleRegion, roleExternalID, region string) (aws.Config, error) {
	// Create base config for role assumption
	var baseCfg aws.Config
	var err error

	if accessKeyID != "" && secretAccessKey != "" {
		var baseCredsProvider aws.CredentialsProvider
		if sessionToken != "" {
			baseCredsProvider = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, sessionToken)
		} else {
			baseCredsProvider = credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")
		}

		baseCfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(roleRegion),
			config.WithCredentialsProvider(baseCredsProvider),
		)
	} else {
		baseCfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(roleRegion))
	}

	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load base AWS config for role assumption: %w", err)
	}

	// Create STS client for role assumption
	stsClient := sts.NewFromConfig(baseCfg)

	// Create assume role credentials provider
	assumeRoleProvider := stscreds.NewAssumeRoleProvider(stsClient, roleARN, func(o *stscreds.AssumeRoleOptions) {
		if roleExternalID != "" {
			o.ExternalID = aws.String(roleExternalID)
		}
		o.RoleSessionName = "bedrock-guardrail-session"
	})

	// Load final config with assumed role credentials for the target region
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(assumeRoleProvider),
	)
	if err != nil {
		return aws.Config{}, fmt.Errorf("failed to load AWS config with assume role: %w", err)
	}

	return cfg, nil
}

// applyBedrockGuardrail calls AWS Bedrock Guardrail ApplyGuardrail API
func (p *AWSBedrockGuardrailPolicy) applyBedrockGuardrail(ctx context.Context, awsCfg aws.Config, guardrailID, guardrailVersion, content string) (*bedrockruntime.ApplyGuardrailOutput, error) {
	// Create Bedrock Runtime client
	client := bedrockruntime.NewFromConfig(awsCfg)

	// Prepare ApplyGuardrail input
	input := &bedrockruntime.ApplyGuardrailInput{
		GuardrailIdentifier: aws.String(guardrailID),
		GuardrailVersion:    aws.String(guardrailVersion),
		Source:              types.GuardrailContentSourceInput,
		Content: []types.GuardrailContentBlock{
			&types.GuardrailContentBlockMemberText{
				Value: types.GuardrailTextBlock{
					Text: aws.String(content),
				},
			},
		},
	}

	// Call ApplyGuardrail API
	output, err := client.ApplyGuardrail(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("ApplyGuardrail API call failed: %w", err)
	}

	return output, nil
}

// evaluateGuardrailResponse processes the AWS Bedrock Guardrail response
func (p *AWSBedrockGuardrailPolicy) evaluateGuardrailResponse(output interface{}, originalContent string, redactPII bool, isRequest bool, metadata map[string]interface{}) (bool, string, error) {
	if output == nil {
		return true, "", fmt.Errorf("AWS Bedrock Guardrails API returned an invalid response")
	}

	outputTyped, ok := output.(*bedrockruntime.ApplyGuardrailOutput)
	if !ok {
		return true, "", fmt.Errorf("invalid output type")
	}

	// Check if guardrail intervened
	if outputTyped.Action == types.GuardrailActionGuardrailIntervened {
		// Check if there are PII entities or sensitive information that was masked
		hasPIIMasking := false
		if len(outputTyped.Assessments) > 0 {
			for _, assessment := range outputTyped.Assessments {
				if assessment.SensitiveInformationPolicy != nil {
					if len(assessment.SensitiveInformationPolicy.PiiEntities) > 0 || len(assessment.SensitiveInformationPolicy.Regexes) > 0 {
						hasPIIMasking = true
						break
					}
				}
			}
		}

		// If PII masking was applied
		if hasPIIMasking {
			if redactPII {
				// Redaction mode: extract redacted content
				redactedContent := p.extractRedactedContent(outputTyped, originalContent)
				return false, redactedContent, nil
			} else if isRequest {
				// Masking mode: process PII entities for masking
				maskedContent, maskedPII := p.processPIIEntitiesForMasking(outputTyped, originalContent)
				if len(maskedPII) > 0 {
					metadata[MetadataKeyPIIEntities] = maskedPII
				}
				return false, maskedContent, nil
			}
		}

		// Other intervention reasons - block by default (content policy, topic policy, word policy violations)
		return true, "", nil // Violation detected, block content
	}

	// Check for no intervention
	if outputTyped.Action == types.GuardrailActionNone {
		return false, "", nil // No violation, continue processing
	}

	// Unexpected response
	return true, "", fmt.Errorf("AWS Bedrock Guardrails returned unexpected response action: %s", string(outputTyped.Action))
}

// processPIIEntitiesForMasking handles PII masking when redactPII is disabled
func (p *AWSBedrockGuardrailPolicy) processPIIEntitiesForMasking(output *bedrockruntime.ApplyGuardrailOutput, originalContent string) (string, map[string]string) {
	if output == nil || len(output.Assessments) == 0 {
		return originalContent, nil
	}

	maskedPII := make(map[string]string)
	updatedContent := originalContent
	counter := 0

	for _, assessment := range output.Assessments {
		if assessment.SensitiveInformationPolicy != nil {
			// Process PII entities
			if len(assessment.SensitiveInformationPolicy.PiiEntities) > 0 {
				for _, entity := range assessment.SensitiveInformationPolicy.PiiEntities {
					if entity.Action == types.GuardrailSensitiveInformationPolicyActionAnonymized {
						match := aws.ToString(entity.Match)
						if match != "" && maskedPII[match] == "" {
							entityType := string(entity.Type)
							replacement := fmt.Sprintf("%s_%04x", entityType, counter)
							updatedContent = strings.ReplaceAll(updatedContent, match, replacement)
							maskedPII[match] = replacement
							counter++
						}
					}
				}
			}

			// Process regex matches
			if len(assessment.SensitiveInformationPolicy.Regexes) > 0 {
				for _, regex := range assessment.SensitiveInformationPolicy.Regexes {
					if regex.Action == types.GuardrailSensitiveInformationPolicyActionAnonymized {
						match := aws.ToString(regex.Match)
						name := aws.ToString(regex.Name)
						if match != "" && maskedPII[match] == "" {
							replacement := fmt.Sprintf("%s_%04x", name, counter)
							updatedContent = strings.ReplaceAll(updatedContent, match, replacement)
							maskedPII[match] = replacement
							counter++
						}
					}
				}
			}
		}
	}

	return updatedContent, maskedPII
}

// extractRedactedContent extracts redacted content from guardrail outputs
func (p *AWSBedrockGuardrailPolicy) extractRedactedContent(output *bedrockruntime.ApplyGuardrailOutput, originalContent string) string {
	redactedText := originalContent
	// Replace all PII entity matches with *****
	if output != nil && len(output.Assessments) > 0 && output.Assessments[0].SensitiveInformationPolicy != nil {
		for _, entity := range output.Assessments[0].SensitiveInformationPolicy.PiiEntities {
			match := aws.ToString(entity.Match)
			if match != "" {
				redactedText = strings.ReplaceAll(redactedText, match, "*****")
			}
		}
		for _, regex := range output.Assessments[0].SensitiveInformationPolicy.Regexes {
			match := aws.ToString(regex.Match)
			if match != "" {
				redactedText = strings.ReplaceAll(redactedText, match, "*****")
			}
		}
	}
	return redactedText
}

// restorePIIInResponse handles PII restoration in responses when redactPII is disabled
func (p *AWSBedrockGuardrailPolicy) restorePIIInResponse(originalContent string, maskedPIIEntities map[string]string) string {
	if maskedPIIEntities == nil || len(maskedPIIEntities) == 0 {
		return originalContent
	}

	transformedContent := originalContent
	for original, placeholder := range maskedPIIEntities {
		if strings.Contains(transformedContent, placeholder) {
			transformedContent = strings.ReplaceAll(transformedContent, placeholder, original)
		}
	}

	return transformedContent
}

// updatePayloadWithMaskedContent updates the original payload by replacing the extracted content
func (p *AWSBedrockGuardrailPolicy) updatePayloadWithMaskedContent(originalPayload []byte, extractedValue, modifiedContent string, jsonPath string) []byte {
	if jsonPath == "" {
		return []byte(modifiedContent)
	}

	var jsonData map[string]interface{}
	if err := json.Unmarshal(originalPayload, &jsonData); err != nil {
		return []byte(modifiedContent)
	}

	err := utils.SetValueAtJSONPath(jsonData, jsonPath, modifiedContent)
	if err != nil {
		return originalPayload
	}

	updatedPayload, err := json.Marshal(jsonData)
	if err != nil {
		return originalPayload
	}

	return updatedPayload
}

// buildErrorResponse builds an error response for both request and response phases
func (p *AWSBedrockGuardrailPolicy) buildErrorResponse(reason string, validationError error, isResponse bool, showAssessment bool, output interface{}) interface{} {
	assessment := p.buildAssessmentObject(reason, validationError, isResponse, showAssessment, output)

	responseBody := map[string]interface{}{
		"code":    GuardrailAPIMExceptionCode,
		"type":    "AWS_BEDROCK_GUARDRAIL",
		"message": assessment,
	}

	bodyBytes, err := json.Marshal(responseBody)
	if err != nil {
		bodyBytes = []byte(fmt.Sprintf(`{"code":%d,"type":"AWS_BEDROCK_GUARDRAIL","message":"Internal error"}`, GuardrailAPIMExceptionCode))
	}

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
func (p *AWSBedrockGuardrailPolicy) buildAssessmentObject(reason string, validationError error, isResponse bool, showAssessment bool, output interface{}) map[string]interface{} {
	assessment := map[string]interface{}{
		"action":               "GUARDRAIL_INTERVENED",
		"interveningGuardrail": "AWS Bedrock Guardrail",
	}

	if isResponse {
		assessment["direction"] = "RESPONSE"
	} else {
		assessment["direction"] = "REQUEST"
	}

	if validationError != nil {
		assessment["actionReason"] = reason
	} else {
		assessment["actionReason"] = "Violation of AWS Bedrock Guardrail detected."
	}

	if showAssessment {
		if validationError != nil {
			assessment["assessments"] = []string{validationError.Error()}
		} else if bedrockOutput, ok := output.(*bedrockruntime.ApplyGuardrailOutput); ok && bedrockOutput != nil {
			if len(bedrockOutput.Assessments) > 0 {
				firstAssessment := p.convertBedrockAssessmentToMap(bedrockOutput.Assessments[0])
				assessment["assessments"] = firstAssessment
			}
		}
	}

	return assessment
}

// convertBedrockAssessmentToMap converts a Bedrock assessment to a map structure
func (p *AWSBedrockGuardrailPolicy) convertBedrockAssessmentToMap(assessment types.GuardrailAssessment) map[string]interface{} {
	assessmentMap := make(map[string]interface{})

	// Handle content policy assessment
	if assessment.ContentPolicy != nil {
		contentPolicy := make(map[string]interface{})
		if len(assessment.ContentPolicy.Filters) > 0 {
			filters := make([]map[string]interface{}, 0, len(assessment.ContentPolicy.Filters))
			for _, filter := range assessment.ContentPolicy.Filters {
				filterMap := map[string]interface{}{
					"action":     string(filter.Action),
					"confidence": string(filter.Confidence),
					"type":       string(filter.Type),
				}
				filters = append(filters, filterMap)
			}
			contentPolicy["filters"] = filters
		}
		assessmentMap["contentPolicy"] = contentPolicy
	}

	// Handle topic policy assessment
	if assessment.TopicPolicy != nil {
		topicPolicy := make(map[string]interface{})
		if len(assessment.TopicPolicy.Topics) > 0 {
			topics := make([]map[string]interface{}, 0, len(assessment.TopicPolicy.Topics))
			for _, topic := range assessment.TopicPolicy.Topics {
				topicMap := map[string]interface{}{
					"action": string(topic.Action),
					"name":   aws.ToString(topic.Name),
					"type":   string(topic.Type),
				}
				topics = append(topics, topicMap)
			}
			topicPolicy["topics"] = topics
		}
		assessmentMap["topicPolicy"] = topicPolicy
	}

	// Handle word policy assessment
	if assessment.WordPolicy != nil {
		wordPolicy := make(map[string]interface{})
		if len(assessment.WordPolicy.CustomWords) > 0 {
			customWords := make([]map[string]interface{}, 0, len(assessment.WordPolicy.CustomWords))
			for _, word := range assessment.WordPolicy.CustomWords {
				wordMap := map[string]interface{}{
					"action": string(word.Action),
					"match":  aws.ToString(word.Match),
				}
				customWords = append(customWords, wordMap)
			}
			wordPolicy["customWords"] = customWords
		}
		if len(assessment.WordPolicy.ManagedWordLists) > 0 {
			managedWords := make([]map[string]interface{}, 0, len(assessment.WordPolicy.ManagedWordLists))
			for _, word := range assessment.WordPolicy.ManagedWordLists {
				wordMap := map[string]interface{}{
					"action": string(word.Action),
					"match":  aws.ToString(word.Match),
					"type":   string(word.Type),
				}
				managedWords = append(managedWords, wordMap)
			}
			wordPolicy["managedWordLists"] = managedWords
		}
		assessmentMap["wordPolicy"] = wordPolicy
	}

	// Handle sensitive information policy assessment
	if assessment.SensitiveInformationPolicy != nil {
		sipPolicy := make(map[string]interface{})
		if len(assessment.SensitiveInformationPolicy.PiiEntities) > 0 {
			piiEntities := make([]map[string]interface{}, 0, len(assessment.SensitiveInformationPolicy.PiiEntities))
			for _, entity := range assessment.SensitiveInformationPolicy.PiiEntities {
				entityMap := map[string]interface{}{
					"action": string(entity.Action),
					"match":  aws.ToString(entity.Match),
					"type":   string(entity.Type),
				}
				piiEntities = append(piiEntities, entityMap)
			}
			sipPolicy["piiEntities"] = piiEntities
		}
		if len(assessment.SensitiveInformationPolicy.Regexes) > 0 {
			regexes := make([]map[string]interface{}, 0, len(assessment.SensitiveInformationPolicy.Regexes))
			for _, regex := range assessment.SensitiveInformationPolicy.Regexes {
				regexMap := map[string]interface{}{
					"action": string(regex.Action),
					"match":  aws.ToString(regex.Match),
					"name":   aws.ToString(regex.Name),
				}
				regexes = append(regexes, regexMap)
			}
			sipPolicy["regexes"] = regexes
		}
		assessmentMap["sensitiveInformationPolicy"] = sipPolicy
	}

	return assessmentMap
}
