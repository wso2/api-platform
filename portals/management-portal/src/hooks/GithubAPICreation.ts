import { useCallback } from "react";

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

/* ---------------- Hook ---------------- */

// Importing APIs from a Git repository (and the supporting Git repository
// browsing endpoints) is no longer supported by the platform. These actions
// now return a "not supported" response instead of calling the backend.
const NOT_SUPPORTED_MESSAGE =
  "Importing APIs from a Git repository is no longer supported.";

export const useGithubAPICreation = () => {
  /** No longer supported: git/repo/fetch-branches */
  const fetchBranches = useCallback(
    async (
      _repoUrl: string,
      _provider: GitProvider = "github",
      _opts?: { signal?: AbortSignal }
    ): Promise<GitBranch[]> => {
      throw new Error(NOT_SUPPORTED_MESSAGE);
    },
    []
  );

  /** No longer supported: git/repo/branch/fetch-content */
  const fetchBranchContent = useCallback(
    async (
      _repoUrl: string,
      _provider: GitProvider,
      _branch: string,
      _opts?: { signal?: AbortSignal }
    ): Promise<GitFetchContentResponse> => {
      throw new Error(NOT_SUPPORTED_MESSAGE);
    },
    []
  );

  /** No longer supported: /api/v0.9/api-projects/import */
  const importApiProject = useCallback(
    async (
      _payload: ImportApiProjectRequest,
      _opts?: { signal?: AbortSignal }
    ): Promise<ApiSummary> => {
      throw new Error(NOT_SUPPORTED_MESSAGE);
    },
    []
  );

  return {
    fetchBranches,
    fetchBranchContent,
    importApiProject,
  };
};
