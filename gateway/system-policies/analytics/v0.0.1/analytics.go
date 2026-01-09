package analytics

import (
	"log/slog"

	policy "github.com/wso2/api-platform/sdk/gateway/policy/v1alpha"
)

const (

)

// AnalyticsPolicy implements the default analytics data collection process.
type AnalyticsPolicy struct{}

var ins = &AnalyticsPolicy{}

func GetPolicy(
	metadata policy.PolicyMetadata,
	params map[string]interface{},
) (policy.Policy, error) {
	return ins, nil
}

// Mode returns the processing mode for this policy
func (a *AnalyticsPolicy) Mode() policy.ProcessingMode {
	// For now analytics will go through all the headers and body to collect the analytics data.
	return policy.ProcessingMode{
		RequestHeaderMode:  policy.HeaderModeProcess, 
		RequestBodyMode:    policy.BodyModeBuffer,
		ResponseHeaderMode: policy.HeaderModeProcess,  
		ResponseBodyMode:   policy.BodyModeBuffer,      
	}
}

// OnRequest performs Analytics collection process during the request phase
func (a *AnalyticsPolicy) OnRequest(ctx *policy.RequestContext, params map[string]interface{}) policy.RequestAction {
	slog.Info("Analytics system policy: OnRequest called")
	return nil
}


// OnRequest performs Analytics collection process during the response phase
func (p *AnalyticsPolicy) OnResponse(ctx *policy.ResponseContext, params map[string]interface{}) policy.ResponseAction {
	slog.Info("Analytics system policy: OnResponse called")
	return nil
}