/*
 * Copyright (c) 2024, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
module.exports = {
    DEV_MODE: 'development',
    IMAGE: 'image',
    STYLE: 'style',
    TEXT: 'text',
    CHARSET_UTF8: 'utf-8',
    FILE_NAME_PARAM: '&fileName=',
    API_ICON: 'api-icon',
    API_TEMPLATE_FILE_NAME: '/template?type=IMAGE&fileName=',
    API_TYPE_QUERY: '/template?type=',
    BASE_URL: 'https://localhost:',
    BASE_URL_NAME: 'baseUrl',
    ORG_ID: 'orgID',
    ORG_IDENTIFIER: 'organizationIdentifier',
    ORG_HANDLE: 'orgHandle',
    ACCESS_TOKEN: 'accessToken',
    REFRESH_TOKEN: 'refreshToken',
    EXCHANGE_TOKEN: 'exchangeToken',
    USER_ID: 'sub',
    BASIC_HEADER: 'basicAuthHeader',
    API_VISIBILITY: {
        PUBLIC: 'PUBLIC',
        PRIVATE: 'PRIVATE'
    },
    API_STATUS: {
        PUBLISHED: "PUBLISHED",
        UNPUBLISHED: "CREATED"
    },
    API_TYPE: {
        MCP: "MCP",
        MCP_ONLY: "MCPSERVERSONLY",
        API_PROXIES: "APISONLY",
        DEFAULT: "DEFAULT",
        WS: "WS"
    },
    DOC_TYPES: {
        DOC_ID: 'DOC_',
        DOCLINK_ID: 'LINK_',
        API_LANDING: 'MARKETING',
        API_DEFINITION: 'API_DEFINITION',
        SCHEMA_DEFINITION: 'SCHEMA_DEFINITION',
        IMAGES: 'IMAGE',
        DOCUMENT: 'DOCUMENT',
        LINK: "DOC_LINK",
        DOCS: {
            HOW_TO: 'HowTo',
            SAMPLES: 'Samples',
            PUBLIC_FORUM: 'PublicForum',
            SUPPORT_FORUM: 'SupportForum',
            OTHER: 'Other',
            API_DEFINITION: 'Specification'
        }
    },
    MIME_TYPES: {
        HTML: 'text/html',
        TEXT: 'text/plain',
        JSON: 'application/json',
        YAML: 'application/x-yaml',
        XML: 'application/xml',
        CSS: 'text/css',
        JAVASCRIPT: 'application/javascript',
        PNG: 'image/png',
        JPEG: 'image/jpeg',
        SVG: 'image/svg+xml',
        PDF: 'application/pdf',
        CONYEMT_TYPE_OCT: 'application/octet-stream',
        CONYEMT_TYPE: 'Content-Type',
        CONTENT_DISPOSITION: 'Content-Disposition',
        Cache_Control: 'Cache-Control',
    },

    SCOPES: {
        ADMIN: 'admin',
        DEVELOPER: 'dev',
    },

    FILE_EXTENSIONS: {
        HTML: '.html',
        JSON: '.json',
        CSS: '.css',
        JAVASCRIPT: '.js',
        PNG: '.png',
        JPEG: '.jpeg',
        JPG: '.jpg',
        SVG: '.svg',
        PDF: '.pdf',
        HBS: '.hbs',
        MD: '.md',
        GIF: '.gif',
        YAML: '.yaml',
        YML: '.yml',
        XML: '.xml'
    },
    KEY_MANAGERS: {
        INTERNAL_KEY_MANAGER: '_internal_key_manager',
        RESIDENT_KEY_MANAGER: 'Resident Key Manager',
        APP_DEV_STS_KEY_MANAGER: '_appdev_sts_key_manager_',
    },
    TOKEN_TYPES: {
        API_KEY: 'API_KEY',
        OAUTH: 'OAUTH',
        BASIC: 'BASIC'
    },
    ROUTE: {
        DEV_PORTAL: '/devportal',
        STYLES: '/styles',
        TECHNICAL_STYLES: '/technical-styles',
        TECHNICAL_SCRIPTS: '/technical-scripts',
        IMAGES: '/images',
        IMAGES_PATH: '/images/',
        DEFAULT: '/',
        MOCK: '/mock',
        API_LISTING_PAGE: '/apis',
        API_FILE_PATH: '/apis/',
        API_LANDING_PAGE_PATH: '/api/',
        API_DOCS_PATH: '/docs/',
        DEVPORTAL_ASSETS_BASE_PATH: '/devportal/organizations/',
        DEVPORTAL_CONFIGURE: '/*/configure',
        DEVPORTAL_ROOT: ['/portal', '/portal/*/edit', '/devportal'],
        DEVPORTAL_API_LISTING: '/*/apis',
        DEVPORTAL_TECHNICAL_PAGES: ['*/application'],
        VIEWS_PATH: "/views/"
    },
    ROLES: {
        ADMIN: 'admin',
        SUBSCRIBER: 'subscriber',
        SUPER_ADMIN: 'superAdmin',
        ROLE_CLAIM: 'roles',
        GROUP_CLAIM: 'groups',
        ORGANIZATION_CLAIM: 'orgClaimName'
    },
    FILE_TYPE: {
        LAYOUT: 'layout',
        TEMPLATE: 'template',
    },
    KEY_TYPE: {
        PRODUCTION: 'PRODUCTION',
        SANDBOX: 'SANDBOX',
    },
    FILE_NAME: {
        MAIN: 'main.hbs',
        PAGE: 'page.hbs',
        API_MD_CONTENT_FILE_NAME: 'apiContent.md',
        API_HBS_CONTENT_FILE_NAME: 'api-content.hbs',
        API_DOC_MD: 'api-doc.md',
        API_DOC_HBS: 'api-doc.hbs',
        API_CONTENT_PARTIAL_NAME: "api-content",
        API_DOC_PARTIAL_NAME: "api-doc",
        API_DEFINITION_FILE_NAME: 'apiDefinition.json',
        SCHEMA_DEFINITION_FILE_NAME: 'schemaDefinition.json',
        API_SPECIFICATION_PATH: 'specification',
        API_DEFINITION_GRAPHQL: 'apiDefinition.graphql',
        API_DEFINITION_XML: 'apiDefinition.xml',
    },
    DEFAULT_SUBSCRIPTION_PLANS: [
        {
            "policyName": "Bronze",
            "description": "Allows 1000 requests per minute",
            "requestCount": 1000,
            "displayName": "Bronze",
            "billingPlan": "FREE"
        },
        {
            "policyName": "Gold",
            "description": "Allows 5000 requests per minute",
            "displayName": "Gold",
            "requestCount": 5000,
            "billingPlan": "FREE"
        },
        {
            "policyName": "Silver",
            "description": "Allows 2000 requests per minute",
            "displayName": "Silver",
            "requestCount": 2000,
            "billingPlan": "FREE"
        },
        {
            "policyName": "Unlimited",
            "description": "Allows unlimited requests",
            "displayName": "Unlimited",
            "requestCount": "Unlimited",
            "billingPlan": "FREE"
        }
    ],
    ERROR_MESSAGE: {
        ORG_NOT_FOUND: "Failed to load organization",
        ORG_CREATE_ERROR: "Error while creating organization",
        ORG_UPDATE_ERROR: "Error while updating organization",
        ORG_DELETE_ERROR: "Erro while deleting organization",
        ORG_CONTENT_NOT_FOUND: "Organization content not found",
        ORG_CONTENT_UPDATE_ERROR: "Error while updating organization content",
        ORG_CONTENT_DELETE_ERROR: "Error while deleting organization content",
        ORG_CONTENT_CREATE_ERROR: "Error while creating organization content",
        API_NOT_FOUND: "Failed to load API",
        API_CREATE_ERROR: "Error while creating API",
        API_UPDATE_ERROR: "Error while updating API",
        API_DELETE_ERROR: "Error while deleting API",
        API_CONTENT_NOT_FOUND: "API content not found",
        API_CONTENT_UPDATE_ERROR: "Error while updating API content",
        API_CONTENT_DELETE_ERROR: "Error while deleting API content",
        API_CONTENT_CREATE_ERROR: "Error while creating API content",
        API_DOCS_LIST_ERROR: "Error while fetching API docs",
        API_LISTING_LOAD_ERROR: "Error while loading API listing",
        IDP_NOT_FOUND: "Failed to load IDP",
        IDP_CREATE_ERROR: "Error while creating IDP",
        IDP_UPDATE_ERROR: "Error while updating IDP",
        IDP_DELETE_ERROR: "Error while deleting IDP",
        API_NOT_IN_ORG: "API does not belong to given organization",
        UNAUTHENTICATED: "Unauthorized access, please log in again",
        FORBIDDEN: "You do not have permission to access this resource",
        PROVIDER_CREATE_ERROR: "Error while creating provider",
        PROVIDER_UPDATE_ERROR: "Error while updating provider",
        PROVIDER_DELETE_ERROR: "Error while deleting provider",
        PROVIDER_FETCH_ERROR: "Error while fetching providers",
        LABEL_DELETE_ERROR: "Error while deleting label",
        LABEL_RETRIEVE_ERROR: "Error while deleting label",
        LABEL_CREATE_ERROR: "Error while creating labels",
        LABEL_UPDATE_ERROR: "Error while updating labels",
        VIEW_CREATE_ERROR: "Error while creating view",
        VIEW_UPDATE_ERROR: "Error while updating view",
        VIEW_DELETE_ERROR: "Error while deleting view",
        VIEW_RETRIEVE_ERROR: "Error while fetching view",
        SUBSCRIPTION_POLICY_CREATE_ERROR: "Error while creating subscription policy",
        SUBSCRIPTION_POLICY_NOT_FOUND: "Subscription policy not found",
        APPLICATION_CREATE_ERROR: "Error while creating application",
        APPLICATION_UPDATE_ERROR: "Error while updating application",
        APPLICATION_DELETE_ERROR: "Error while deleting application",
        APPLICATION_RETRIEVE_ERROR: "Error while fetching application",
        SUBSCRIPTION_CREATE_ERROR: "Error while creating subscription",
        SUBSCRIPTION_RETRIEVE_ERROR: "Error while retrieving subscription",
        SUBSCRIPTION_DELETE_ERROR: "Error while deleting subscription",
        KEY_MAPPING_CREATE_ERROR: "Error while creating key mapping",
        KEY_MAPPING_RETRIEVE_ERROR: "Error while retrieving key mapping",
        KEY_MAPPING_DELETE_ERROR: "Error while deleting key mapping",
        ERR_SUB_EXIST: "ERR_SUB_EXIST",
        UNAUTHORIZED_ORG: "You are not authorized to access this organization",
        UNAUTHORIZED_API: "You are not authorized to access this API",
        API_NOT_FOUND: "Requested API not found",
        COMMON_AUTH_ERROR_MESSAGE: "User is not authenticated to perform this request",
        COMMON_ERROR_MESSAGE: "Oops! Something went wrong",
        COMMON_PAGE_NOT_FOUND_ERROR_MESSAGE: "Requested page not found!"
    },
    REDIS_CONSTANTS: {
        SDK_PROGRESS_CHANNEL: 'sdk-progress',
        KEY_PREFIX: 'sdkfiles:',
        CONNECTION_STATES: {
            CONNECTED: 'connected',
            DISCONNECTED: 'disconnected',
            CONNECTING: 'connecting',
            OFFLINE: 'offline'
        },
        ERRORS: {
            CONNECTION_CLOSED: 'Connection is closed',
            COMMAND_TIMEOUT: 'Command timed out',
            CONNECTION_LOST: 'Connection lost',
            SOCKET_CLOSED: 'Socket closed unexpectedly',
            PIPELINE_TIMEOUT: 'Redis pipeline timeout',
            STORE_FAILED: 'Failed to store file in Redis',
            FILE_NOT_EXIST: 'File does not exist',
            FILE_NOT_FOUND: 'File not found',
            MAX_ATTEMPTS_REACHED: 'Max reconnection attempts exceeded'
        }
    },
    ERROR_CODE: {
        401: "Unauthenticated",
        403: "Forbidden",
        404: "Not Found",
        500: "Internal Server Error"
    },
    CUSTOMIZABLE_FILES: [
        'header',
        'main',
        'home',
        'api-content',
    ]
}