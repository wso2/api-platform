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
// Devportal API base segment and version — single source of truth for the
// invocation prefix `/api/v0.9`. Change these two to bump the base segment
// (e.g. devportalv2) or version (e.g. v2) everywhere.
const DEVPORTAL_BASE_SEGMENT = 'api';
const DEVPORTAL_VERSION = 'v0.9';
// Express route prefix for devportal routes, e.g. '/api/v0.9'
const DEVPORTAL_BASE_PATH = `/${DEVPORTAL_BASE_SEGMENT}/${DEVPORTAL_VERSION}`;
// Builder for the devportal base path used in server-side URL generation.
// The orgId argument is accepted for backward-compatibility but not used —
// org context is resolved from the token/session, not the URL.
const devportalOrgPath = (_orgId) => `/${DEVPORTAL_BASE_SEGMENT}/${DEVPORTAL_VERSION}`;

module.exports = {
    DEVPORTAL_API: {
        BASE_SEGMENT: DEVPORTAL_BASE_SEGMENT,
        VERSION: DEVPORTAL_VERSION,
        BASE_PATH: DEVPORTAL_BASE_PATH,
        orgPath: devportalOrgPath,
    },
    IMAGE: 'image',
    STYLE: 'style',
    TEXT: 'text',
    CHARSET_UTF8: 'utf-8',
    FILE_NAME_PARAM: '&fileName=',
    API_ICON: 'api-icon',
    API_TEMPLATE_FILE_NAME: '/assets?type=IMAGE&fileName=',
    API_TYPE_QUERY: '/assets?type=',
    BASE_URL: 'https://localhost:',
    BASE_URL_NAME: 'baseUrl',
    ORG_ID: 'orgId',
    ORG_IDENTIFIER: 'idpRefId',
    ORG_HANDLE: 'orgHandle',
    ACCESS_TOKEN: 'accessToken',
    REFRESH_TOKEN: 'refreshToken',
    USER_ID: 'sub',
    SYSTEM_ACTOR: 'system',
    BASIC_HEADER: 'basicAuthHeader',
    API_STATUS: {
        PUBLISHED: "PUBLISHED",
        DEPRECATED: "DEPRECATED"
    },
    API_WORKFLOW_STATUS: {
        DRAFT: "DRAFT",
        PUBLISHED: "PUBLISHED",
    },
    API_WORKFLOW_CONTENT_TYPE: {
        ARAZZO: "ARAZZO",
        MD: "MD",
    },
    AGENT_VISIBILITY: {
        VISIBLE: "VISIBLE",
        HIDDEN: "HIDDEN",
    },
    SUBSCRIPTION_STATUS: {
        ACTIVE: "ACTIVE",
        INACTIVE: "INACTIVE",
    },
    API_KEY_STATUS: {
        ACTIVE: "ACTIVE",
        REVOKED: "REVOKED",
    },
    API_TYPE: {
        REST: "RestApi",
        SOAP: "SOAP",
        MCP: "Mcp",
        WS: "WS",
        WEBSUB: "WebSubApi",
        GRAPHQL: "GRAPHQL",
    },
    DEVPORTAL_MODE: {
        DEFAULT: "DEFAULT",
        MCP_SERVERS_ONLY: "MCP_SERVERS_ONLY",
        APIS_ONLY: "APIS_ONLY",
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
        DEVPORTAL_CONFIGURE: ['/*/settings', '/*/views/*/settings'],
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
        LLMS_CONFIG: 'llms-config',
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
        API_DEFINITION_YAML_FILE_NAME: 'apiDefinition.yaml',
        SCHEMA_DEFINITION_FILE_NAME: 'schemaDefinition.json',
        SCHEMA_DEFINITION_YAML_FILE_NAME: 'schemaDefinition.yaml',
        API_SPECIFICATION_PATH: 'specification',
        API_DEFINITION_GRAPHQL: 'apiDefinition.graphql',
        API_DEFINITION_XML: 'apiDefinition.xml',
        LLMS_CONFIG: 'llms-config.json',
    },
    ARTIFACT_DIR: {
        WEB: 'web',
        DOCS: 'docs',
    },
    DEFAULT_SUBSCRIPTION_PLANS: [
        {
            "handle": "Bronze",
            "name": "Bronze",
            "description": "Allows 1000 requests per minute",
            "limits": [{ "limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "timeAmount": 1, "limitCount": 1000 }],
        },
        {
            "handle": "Silver",
            "name": "Silver",
            "description": "Allows 2000 requests per minute",
            "limits": [{ "limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "timeAmount": 1, "limitCount": 2000 }],
        },
        {
            "handle": "Gold",
            "name": "Gold",
            "description": "Allows 5000 requests per minute",
            "limits": [{ "limitType": "REQUEST_COUNT", "timeUnit": "MINUTE", "timeAmount": 1, "limitCount": 5000 }],
        },
        {
            "handle": "Unlimited",
            "name": "Unlimited",
            "description": "Allows unlimited requests",
            "limits": [{ "limitType": "REQUEST_COUNT", "timeUnit": null, "timeAmount": 1, "limitCount": -1 }],
        },
        {
            "handle": "AsyncUnlimited",
            "name": "AsyncUnlimited",
            "description": "Allows unlimited requests for Async APIs",
            "limits": [{ "limitType": "EVENT_COUNT", "timeUnit": null, "timeAmount": 1, "limitCount": -1 }],
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
        LABEL_DELETE_ERROR: "Error while deleting label",
        LABEL_RETRIEVE_ERROR: "Error while deleting label",
        LABEL_CREATE_ERROR: "Error while creating labels",
        LABEL_UPDATE_ERROR: "Error while updating labels",
        VIEW_CREATE_ERROR: "Error while creating view",
        VIEW_UPDATE_ERROR: "Error while updating view",
        VIEW_DELETE_ERROR: "Error while deleting view",
        VIEW_RETRIEVE_ERROR: "Error while fetching view",
        SUBSCRIPTION_PLAN_CREATE_ERROR: "Error while creating subscription plan",
        SUBSCRIPTION_PLAN_NOT_FOUND: "Subscription plan not found",
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
        KEY_MANAGER_CREATE_ERROR: "Error while creating key manager",
        KEY_MANAGER_UPDATE_ERROR: "Error while updating key manager",
        KEY_MANAGER_DELETE_ERROR: "Error while deleting key manager",
        KEY_MANAGER_RETRIEVE_ERROR: "Error while retrieving key manager",
        KEY_MANAGER_NOT_FOUND: "Key manager not found",
        WEBHOOK_SUBSCRIBER_CREATE_ERROR: "Error while creating webhook subscriber",
        WEBHOOK_SUBSCRIBER_UPDATE_ERROR: "Error while updating webhook subscriber",
        WEBHOOK_SUBSCRIBER_DELETE_ERROR: "Error while deleting webhook subscriber",
        WEBHOOK_SUBSCRIBER_RETRIEVE_ERROR: "Error while retrieving webhook subscriber",
        WEBHOOK_SUBSCRIBER_NOT_FOUND: "Webhook subscriber not found",
        ERR_SUB_EXIST: "ERR_SUB_EXIST",
        ERR_KEY_EXIST: "ERR_KEY_EXIST",
        UNAUTHORIZED_ORG: "You are not authorized to access this organization",
        UNAUTHORIZED_API: "You are not authorized to access this API",
        API_NOT_FOUND: "Requested API not found",
        API_WORKFLOW_CREATE_ERROR: "Error while creating API workflow",
        API_WORKFLOW_UPDATE_ERROR: "Error while updating API workflow",
        API_WORKFLOW_DELETE_ERROR: "Error while deleting API workflow",
        API_WORKFLOW_RETRIEVE_ERROR: "Error while fetching API workflow",
        API_WORKFLOW_NOT_FOUND: "API workflow not found",
        COMMON_AUTH_ERROR_MESSAGE: "User is not authenticated to perform this request",
        COMMON_ERROR_MESSAGE: "Oops! Something went wrong",
        COMMON_PAGE_NOT_FOUND_ERROR_MESSAGE: "Requested page not found!"
    },
    ERROR_CODE: {
        400: "Bad Request",
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
        'apis-md',
        'api-landing-md',
        'llms-txt',
    ],
    DEFAULT_PROFILE_IMAGE_URL: 'https://raw.githubusercontent.com/wso2/docs-bijira/refs/heads/main/en/devportal-theming/profile.svg',
}
