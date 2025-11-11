package dto

import "errors"

// Organization-related errors
var (
	ErrOrganizationAlreadyExists  = errors.New("organization already exists")
	ErrOrganizationNotFound       = errors.New("organization not found")
	ErrOrganizationCreationFailed = errors.New("organization creation failed")
	ErrOrganizationUpdateFailed   = errors.New("organization update failed")
	ErrOrganizationDeletionFailed = errors.New("organization deletion failed")
)

// API-related errors
var (
	ErrAPIAlreadyExists   = errors.New("API already exists")
	ErrAPINotFound        = errors.New("API not found")
	ErrAPICreationFailed  = errors.New("API creation failed")
	ErrAPIUpdateFailed    = errors.New("API update failed")
	ErrAPIDeletionFailed  = errors.New("API deletion failed")
	ErrAPIPublishFailed   = errors.New("API publish failed")
	ErrAPIUnpublishFailed = errors.New("API unpublish failed")
)

// Subscription Policy-related errors
var (
	ErrSubscriptionPolicyAlreadyExists  = errors.New("subscription policy already exists")
	ErrSubscriptionPolicyNotFound       = errors.New("subscription policy not found")
	ErrSubscriptionPolicyCreationFailed = errors.New("subscription policy creation failed")
	ErrSubscriptionPolicyUpdateFailed   = errors.New("subscription policy update failed")
	ErrSubscriptionPolicyDeletionFailed = errors.New("subscription policy deletion failed")
)

// DevPortal connection and general errors
var (
	ErrDevPortalConnectionFailed     = errors.New("devportal connection failed")
	ErrDevPortalTimeout              = errors.New("devportal request timeout")
	ErrDevPortalAuthenticationFailed = errors.New("devportal authentication failed")
	ErrDevPortalInvalidRequest       = errors.New("devportal invalid request")
	ErrDevPortalServerError          = errors.New("devportal server error")
	ErrDevPortalServiceUnavailable   = errors.New("devportal service unavailable")
)

// Template-related errors
var (
	ErrTemplateNotFound       = errors.New("template not found")
	ErrTemplateUploadFailed   = errors.New("template upload failed")
	ErrTemplateUpdateFailed   = errors.New("template update failed")
	ErrTemplateDeletionFailed = errors.New("template deletion failed")
)

// Label-related errors
var (
	ErrLabelNotFound       = errors.New("label not found")
	ErrLabelCreationFailed = errors.New("label creation failed")
	ErrLabelUpdateFailed   = errors.New("label update failed")
	ErrLabelDeletionFailed = errors.New("label deletion failed")
)

// View-related errors
var (
	ErrViewNotFound       = errors.New("view not found")
	ErrViewCreationFailed = errors.New("view creation failed")
	ErrViewUpdateFailed   = errors.New("view update failed")
	ErrViewDeletionFailed = errors.New("view deletion failed")
)
