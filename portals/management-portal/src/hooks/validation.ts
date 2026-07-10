import { useCallback } from "react";

/** ----- Types ----- */

export type GitProvider = "github";

export type GithubProjectValidationRequest = {
  repoUrl: string;
  provider: GitProvider; // "github"
  branch: string;
  path: string; // e.g., "apis/petstore-api"
};

export type GithubProjectValidationOK = {
  isAPIProjectValid: true;
  isAPIConfigValid: true;
  isAPIDefinitionValid: true;
  api: Record<string, unknown>; // backend returns API summary shape
};

export type GithubProjectValidationErr = {
  isAPIProjectValid: false;
  isAPIConfigValid: boolean; // server sends booleans
  isAPIDefinitionValid: boolean;
  errors: string[];
};

export type GithubProjectValidationResponse =
  | GithubProjectValidationOK
  | GithubProjectValidationErr;

export type OpenApiValidationOK = {
  isAPIDefinitionValid: true;
  api: Record<string, unknown>;
};

export type OpenApiValidationErr = {
  isAPIDefinitionValid: false;
  errors: string[];
};

export type OpenApiValidationResponse =
  | OpenApiValidationOK
  | OpenApiValidationErr;

/** NEW: Uniqueness validation (name+version / identifier) */

export type ApiValidateError = {
  code: string;
  message: string;
};

export type ApiUniquenessValidationResponse = {
  valid: boolean;
  error: ApiValidateError | null;
};

export type ApiNameVersionValidationRequest = {
  name: string;
  version: string;
};

/** ----- Hook ----- */

export const useGithubProjectValidation = () => {
  /**
   * No longer supported: /api/v0.9/api-projects/validate
   * Validating API projects from a Git repository has been removed from the
   * platform. Instead of calling the backend, return a "not supported"
   * validation response.
   */
  const validateGithubApiProject = useCallback(
    async (
      _payload: GithubProjectValidationRequest,
      _opts?: { signal?: AbortSignal }
    ): Promise<GithubProjectValidationResponse> => {
      return {
        isAPIProjectValid: false,
        isAPIConfigValid: false,
        isAPIDefinitionValid: false,
        errors: [
          "Validating API projects from a Git repository is no longer supported.",
        ],
      };
    },
    []
  );

  return { validateGithubApiProject };
};

export const useOpenApiValidation = () => {
  /**
   * No longer supported: POST /api/v0.9/rest-apis/validate-openapi
   * Validating an OpenAPI definition has been removed from the platform.
   * Instead of calling the backend, return a "not supported" validation
   * response.
   */
  const validateOpenApiUrl = useCallback(
    async (
      _url: string,
      _opts?: { signal?: AbortSignal }
    ): Promise<OpenApiValidationResponse> => {
      return {
        isAPIDefinitionValid: false,
        errors: ["Validating an OpenAPI definition is no longer supported."],
      };
    },
    []
  );

  const validateOpenApiFile = useCallback(
    async (
      _file: File,
      _opts?: { signal?: AbortSignal }
    ): Promise<OpenApiValidationResponse> => {
      return {
        isAPIDefinitionValid: false,
        errors: ["Validating an OpenAPI definition is no longer supported."],
      };
    },
    []
  );

  return { validateOpenApiUrl, validateOpenApiFile };
};

/** API uniqueness validation hook */
export const useApiUniquenessValidation = () => {
  /**
   * No longer supported: GET /api/v0.9/apis/validate
   * Checking API name/version or identifier uniqueness has been removed from
   * the platform. Instead of calling the backend, return a "not supported"
   * validation response.
   */
  const validateApiNameVersion = useCallback(
    async (
      _payload: ApiNameVersionValidationRequest,
      _opts?: { signal?: AbortSignal }
    ): Promise<ApiUniquenessValidationResponse> => {
      return {
        valid: false,
        error: {
          code: "not-supported",
          message: "Validating API name & version is no longer supported.",
        },
      };
    },
    []
  );

  const validateApiIdentifier = useCallback(
    async (
      _identifier: string,
      _opts?: { signal?: AbortSignal }
    ): Promise<ApiUniquenessValidationResponse> => {
      return {
        valid: false,
        error: {
          code: "not-supported",
          message: "Validating API identifier is no longer supported.",
        },
      };
    },
    []
  );

  return { validateApiNameVersion, validateApiIdentifier };
};
