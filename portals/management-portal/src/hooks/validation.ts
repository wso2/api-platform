import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

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

/** ----- Helpers ----- */
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


const parseError = async (res: Response) => {
  let body = "";
  try {
    body = await res.text();
    try {
      const json = JSON.parse(body);
      if (json?.message)
        return `${res.status} ${res.statusText} — ${json.message}`;
      if (json?.error?.message)
        return `${res.status} ${res.statusText} — ${json.error.message}`;
    } catch {
      /* not JSON */
    }
  } catch {
    /* ignore */
  }
  return `${res.status} ${res.statusText}${body ? ` ${body}` : ""}`;
};

const authedFetch = async (path: string, init?: RequestInit) => {
  const { token, baseUrl } = getApiConfig();
  return fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...(init?.headers || {}),
    },
  });
};

/** ----- Hook ----- */

export const useGithubProjectValidation = () => {
  /** POST: /api/v1/validate/api-project */
  const validateGithubApiProject = useCallback(
    async (
      payload: GithubProjectValidationRequest,
      opts?: { signal?: AbortSignal }
    ): Promise<GithubProjectValidationResponse> => {
      const res = await authedFetch(`/api/v1/validate/api-project`, {
        method: "POST",
        body: JSON.stringify(payload),
        signal: opts?.signal,
      });

      if (!res.ok) {
        throw new Error(
          `Failed to validate API project: ${await parseError(res)}`
        );
      }

      const data = (await res.json()) as GithubProjectValidationResponse;
      return data;
    },
    []
  );

  return { validateGithubApiProject };
};

export const useOpenApiValidation = () => {
  const validateOpenApiUrl = useCallback(
    async (
      url: string,
      opts?: { signal?: AbortSignal }
    ): Promise<OpenApiValidationResponse> => {
      const { token, baseUrl } = getApiConfig();

      const formData = new FormData();
      formData.append("url", url);

      const res = await fetch(`${baseUrl}/api/v1/validate/open-api`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
        signal: opts?.signal,
      });

      if (!res.ok) {
        throw new Error(
          `Failed to validate OpenAPI from URL: ${await parseError(res)}`
        );
      }

      const data = (await res.json()) as OpenApiValidationResponse;
      return data;
    },
    []
  );

  const validateOpenApiFile = useCallback(
    async (
      file: File,
      opts?: { signal?: AbortSignal }
    ): Promise<OpenApiValidationResponse> => {
      const { token, baseUrl } = getApiConfig();

      const formData = new FormData();
      formData.append("definition", file);

      const res = await fetch(`${baseUrl}/api/v1/validate/open-api`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
        signal: opts?.signal,
      });

      if (!res.ok) {
        throw new Error(
          `Failed to validate OpenAPI file: ${await parseError(res)}`
        );
      }

      const data = (await res.json()) as OpenApiValidationResponse;
      return data;
    },
    []
  );

  return { validateOpenApiUrl, validateOpenApiFile };
};
