/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_PLATFORM_API_BASE_URL?: string;
  readonly VITE_PLATFORM_API_VERSION?: string;
  readonly VITE_MOESIF_APP_URL?: string;
  readonly VITE_MOESIF_BASIC_INSIGHTS_URL?: string;
  readonly VITE_ENVIRONMENT_NAME?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
