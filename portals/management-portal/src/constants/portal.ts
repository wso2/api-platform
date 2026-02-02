// src/constants/portal.ts
export const PORTAL_CONSTANTS = {
  // Default values
  DEFAULT_PORTAL_URL: 'https://portal.example.com',
  DEFAULT_USER_AUTH_LABEL: 'Asgardeo Thunder',
  DEFAULT_AUTH_STRATEGY_LABEL: 'Auth-Key',
  DEFAULT_VISIBILITY_LABEL: 'Private',
  DEFAULT_HEADER_KEY_NAME: 'x-wso2-api-key',
  DEFAULT_LOGO_ALT: 'Portal logo',
  API_KEY_MASK: '**********',

  // Portal types
  PORTAL_TYPES: {
    PRIVATE: 'private',
    PUBLIC: 'public',
  } as const,

  // Navigation modes
  MODES: {
    LIST: 'list',
    FORM: 'form',
    EDIT: 'edit',
    THEME: 'theme',
  } as const,

  // Status labels
  STATUS_LABELS: {
    ACTIVE: 'Active',
    INACTIVE: 'Inactive',
    ACTIVATED: '✓ Activated',
    ACTIVATE_PORTAL: 'Activate Developer Portal',
  } as const,

  // Messages
  MESSAGES: {
    PORTAL_CREATED: 'Developer Portal created successfully.',
    PORTAL_ACTIVATED: 'Developer Portal activated successfully.',
    ACTIVATION_FAILED: 'Failed to activate Developer Portal.',
    CREATION_FAILED: 'Failed to create Developer Portal.',
    UPDATE_FAILED: 'Failed to update Developer Portal.',
    FETCH_DEVPORTALS_FAILED: 'Failed to fetch devportals',
    CREATE_DEVPORTAL_FAILED: 'Failed to create devportal',
    UPDATE_DEVPORTAL_FAILED: 'Failed to update devportal',
    DELETE_DEVPORTAL_FAILED: 'Failed to delete devportal',
    FETCH_PORTAL_DETAILS_FAILED: 'Failed to fetch portal details',
    ACTIVATE_DEVPORTAL_FAILED: 'Failed to activate devportal',
    PUBLISH_FAILED: 'Failed to publish',
    NO_PORTAL_SELECTED: 'No portal selected',
    PROVIDE_API_NAME_AND_URL: 'Please provide API Name and Production URL',
    PUBLISH_THEME_FAILED: 'Failed to publish theme',
    PROMO_ACTION_FAILED: 'Promo action failed',
    REFRESH_PUBLICATIONS_FAILED: 'Failed to refresh publications after publish',
    API_PUBLISH_CONTEXT_ERROR: 'useApiPublishing must be used within an ApiPublishProvider',
    LOADING_ERROR: 'An error occurred while loading developer portals.',
    URL_NOT_AVAILABLE:
      'Portal URL is not available until the portal is activated',
    OPEN_PORTAL_URL: 'Open portal URL',
  } as const,

  // Accessibility
  ARIA_LABELS: {
    EDIT_PORTAL: 'Edit portal details',
    ACTIVATE_PORTAL: 'Activate developer portal',
    OPEN_PORTAL_URL: 'Open portal URL in new tab',
    BACK_TO_LIST: 'Back to portal list',
    CREATE_PORTAL: 'Create new developer portal',
  } as const,
} as const;

// Type helpers
export type PortalType =
  (typeof PORTAL_CONSTANTS.PORTAL_TYPES)[keyof typeof PORTAL_CONSTANTS.PORTAL_TYPES];
export type PortalMode =
  (typeof PORTAL_CONSTANTS.MODES)[keyof typeof PORTAL_CONSTANTS.MODES];
