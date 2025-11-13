import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

/* ---------------- Types ---------------- */

export type GitProvider = "github";

export type GitBranch = {
  name: string;
  isDefault: boolean; // normalized from "true"/"false" string
};

export type GitBranchesResponse = {
  repoUrl: string;
  branches: Array<{ name: string; isDefault: string | boolean }>;
};

export type GitTreeItemType = "tree" | "blob";

export type GitTreeItem = {
  path: string; // full path (e.g., "apis/petstore-api/petstore.yaml")
  subPath: string; // path relative to parent ("petstore.yaml")
  children: GitTreeItem[];
  type: GitTreeItemType;
};

export type GitFetchContentResponse = {
  repoUrl: string;
  branch: string;
  items: GitTreeItem[];
  totalItems?: number;
  maxDepth?: number;
  requestedDepth?: number;
};

export type ImportApiProjectRequest = {
  repoUrl: string;
  provider: GitProvider;
  branch: string;
  path: string; // e.g., "apis/test-api"
  api: Record<string, unknown>; // pass through as-is; backend owns validation
};

export type ApiSummary = {
  id: string;
  name: string;
  displayName?: string;
  description?: string;
  context: string;
  version: string;
  provider?: string;
  projectId: string;
  organizationId?: string;
  createdAt?: string;
  updatedAt?: string;
  lifeCycleStatus?: string;
  type?: string;
  transport?: string[];
  ["backend-services"]?: unknown[];
  operations?: unknown[];
};

/* ---------------- Helpers ---------------- */

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
      /* not json */
    }
  } catch {
    /* ignore */
  }
  return `${res.status} ${res.statusText}${body ? ` ${body}` : ""}`;
};

const authedFetch = async (path: string, init?: RequestInit) => {
  const { token, baseUrl } = getApiConfig();
  const res = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      Authorization: `Bearer ${token}`,
      ...(init?.headers || {}),
    },
  });
  return res;
};

const normalizeBranches = (resp: GitBranchesResponse): GitBranch[] =>
  (resp.branches ?? []).map((b) => ({
    name: b.name,
    isDefault:
      typeof b.isDefault === "boolean"
        ? b.isDefault
        : String(b.isDefault).toLowerCase() === "true",
  }));

/* ---------------- Hook ---------------- */

export const useGithubAPICreation = () => {
  /** POST: git/repo/fetch-branches */
  const fetchBranches = useCallback(
    async (
      repoUrl: string,
      provider: GitProvider = "github",
      opts?: { signal?: AbortSignal }
    ): Promise<GitBranch[]> => {
      const res = await authedFetch(`/api/v1/git/repo/fetch-branches`, {
        method: "POST",
        body: JSON.stringify({ repoUrl, provider }),
        signal: opts?.signal,
      });
      if (!res.ok) {
        throw new Error(`Failed to fetch branches: ${await parseError(res)}`);
      }
      const json = (await res.json()) as GitBranchesResponse;
      return normalizeBranches(json);
    },
    []
  );

  /** POST: git/repo/branch/fetch-content */
  const fetchBranchContent = useCallback(
    async (
      repoUrl: string,
      provider: GitProvider,
      branch: string,
      opts?: { signal?: AbortSignal }
    ): Promise<GitFetchContentResponse> => {
      const res = await authedFetch(`/api/v1/git/repo/branch/fetch-content`, {
        method: "POST",
        body: JSON.stringify({ repoUrl, provider, branch }),
        signal: opts?.signal,
      });
      if (!res.ok) {
        throw new Error(
          `Failed to fetch branch content: ${await parseError(res)}`
        );
      }
      const json = (await res.json()) as GitFetchContentResponse;
      return json;
    },
    []
  );

  /** POST: /api/v1/import/api-project */
  const importApiProject = useCallback(
    async (
      payload: ImportApiProjectRequest,
      opts?: { signal?: AbortSignal }
    ): Promise<ApiSummary> => {
      const res = await authedFetch(`/api/v1/import/api-project`, {
        method: "POST",
        body: JSON.stringify(payload),
        signal: opts?.signal,
      });
      if (!res.ok) {
        throw new Error(
          `Failed to import API project: ${await parseError(res)}`
        );
      }
      const json = (await res.json()) as ApiSummary;
      return json;
    },
    []
  );

  return {
    fetchBranches,
    fetchBranchContent,
    importApiProject,
  };
};
