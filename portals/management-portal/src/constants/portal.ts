// src/constants/portal.ts
export const PORTAL_CONSTANTS = {
  // Default values
  DEFAULT_PORTAL_URL: 'https://portal.example.com',
  DEFAULT_USER_AUTH_LABEL: 'Asgardeo Thunder',
  DEFAULT_AUTH_STRATEGY_LABEL: 'Auth-Key',
  DEFAULT_VISIBILITY_LABEL: 'Private',
  DEFAULT_HEADER_KEY_NAME: 'x-wso2-api-key',
  DEFAULT_LOGO_ALT: 'Portal logo',

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
    LOADING_ERROR: 'An error occurred while loading developer portals.',
    URL_NOT_AVAILABLE: 'Portal URL is not available until the portal is activated',
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
export type PortalType = typeof PORTAL_CONSTANTS.PORTAL_TYPES[keyof typeof PORTAL_CONSTANTS.PORTAL_TYPES];
export type PortalMode = typeof PORTAL_CONSTANTS.MODES[keyof typeof PORTAL_CONSTANTS.MODES];