package dto

// OrganizationCreateRequest is the payload used to create an organization in DevPortal.
type OrganizationCreateRequest struct {
	OrgID                  string                 `json:"orgId,omitempty"`
	OrgName                string                 `json:"orgName"`
	OrgHandle              string                 `json:"orgHandle"`
	OrganizationIdentifier string                 `json:"organizationIdentifier"`
	BusinessOwner          string                 `json:"businessOwner,omitempty"`
	BusinessOwnerContact   string                 `json:"businessOwnerContact,omitempty"`
	BusinessOwnerEmail     string                 `json:"businessOwnerEmail,omitempty"`
	RoleClaimName          string                 `json:"roleClaimName,omitempty"`
	GroupsClaimName        string                 `json:"groupsClaimName,omitempty"`
	OrganizationClaimName  string                 `json:"organizationClaimName,omitempty"`
	AdminRole              string                 `json:"adminRole,omitempty"`
	SubscriberRole         string                 `json:"subscriberRole,omitempty"`
	SuperAdminRole         string                 `json:"superAdminRole,omitempty"`
	OrgConfig              map[string]interface{} `json:"orgConfig,omitempty"`
}

// OrganizationResponse is the representation returned by DevPortal for an organization.
type OrganizationResponse struct {
	OrgID                  string                 `json:"orgId"`
	OrgName                string                 `json:"orgName"`
	OrgHandle              string                 `json:"orgHandle"`
	OrganizationIdentifier string                 `json:"organizationIdentifier"`
	BusinessOwner          string                 `json:"businessOwner,omitempty"`
	BusinessOwnerContact   string                 `json:"businessOwnerContact,omitempty"`
	BusinessOwnerEmail     string                 `json:"businessOwnerEmail,omitempty"`
	RoleClaimName          string                 `json:"roleClaimName,omitempty"`
	GroupsClaimName        string                 `json:"groupsClaimName,omitempty"`
	OrganizationClaimName  string                 `json:"organizationClaimName,omitempty"`
	AdminRole              string                 `json:"adminRole,omitempty"`
	SubscriberRole         string                 `json:"subscriberRole,omitempty"`
	SuperAdminRole         string                 `json:"superAdminRole,omitempty"`
	OrgConfiguration       map[string]interface{} `json:"orgConfiguration,omitempty"`
}

// OrganizationUpdateRequest contains fields allowed for updates.
type OrganizationUpdateRequest struct {
	OrgName       *string                `json:"orgName,omitempty"`
	BusinessOwner *string                `json:"businessOwner,omitempty"`
	OrgConfig     map[string]interface{} `json:"orgConfig,omitempty"`
}

// OrganizationListResponse is the JSON array returned by the DevPortal for a list
type OrganizationListResponse []OrganizationResponse
