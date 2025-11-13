import React, {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  useGithubAPICreation,
  type GitProvider,
  type GitBranch,
  type GitTreeItem,
  type GitFetchContentResponse,
  type ImportApiProjectRequest,
  type ApiSummary,
} from "../hooks/GithubAPICreation";

/* ---------------- Context Types ---------------- */

type BranchCache = Record<string /* repoUrl */, GitBranch[]>;
type ContentCache = Record<
  string /* repoUrl#branch */,
  GitFetchContentResponse
>;

type GithubAPICreationContextValue = {
  provider: GitProvider;
  repoUrl: string;
  setProvider: (p: GitProvider) => void;
  setRepoUrl: (url: string) => void;

  branches: GitBranch[];
  selectedBranch: string | null;
  setSelectedBranch: (branch: string | null) => void;
  content: GitFetchContentResponse | null;

  /** UI selections inside the tree (e.g., "apis/test-api") */
  selectedPath: string | null;
  setSelectedPath: (p: string | null) => void;

  loading: boolean;
  error: string | null;

  /** Data loaders with simple caching */
  loadBranches: (
    repo?: string,
    opts?: { force?: boolean }
  ) => Promise<GitBranch[]>;
  loadBranchContent: (
    branch?: string,
    opts?: { force?: boolean }
  ) => Promise<GitFetchContentResponse>;

  /** Import using current selections (repoUrl, provider, selectedBranch, selectedPath) */
  importSelectedApiProject: (
    api: ImportApiProjectRequest["api"]
  ) => Promise<ApiSummary>;
};

const GithubAPICreationContext = createContext<
  GithubAPICreationContextValue | undefined
>(undefined);

type Props = { children: ReactNode };

/* ---------------- Provider ---------------- */

export const GithubAPICreationProvider: React.FC<Props> = ({ children }) => {
  const { fetchBranches, fetchBranchContent, importApiProject } =
    useGithubAPICreation();

  const [provider, setProvider] = useState<GitProvider>("github");
  const [repoUrl, setRepoUrl] = useState<string>("");

  const [branchesByRepo, setBranchesByRepo] = useState<BranchCache>({});
  const [contentByRepoBranch, setContentByRepoBranch] = useState<ContentCache>(
    {}
  );

  const [selectedBranch, setSelectedBranch] = useState<string | null>(null);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);

  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  const mountedRef = useRef(true);
  const pendingRef = useRef(0);

  const begin = () => {
    pendingRef.current += 1;
    setLoading(true);
  };
  const end = () => {
    pendingRef.current = Math.max(0, pendingRef.current - 1);
    if (pendingRef.current === 0 && mountedRef.current) setLoading(false);
  };

  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const branches = useMemo<GitBranch[]>(() => {
    if (!repoUrl) return [];
    return branchesByRepo[repoUrl] ?? [];
  }, [branchesByRepo, repoUrl]);

  const content = useMemo<GitFetchContentResponse | null>(() => {
    if (!repoUrl || !selectedBranch) return null;
    return contentByRepoBranch[`${repoUrl}#${selectedBranch}`] ?? null;
  }, [contentByRepoBranch, repoUrl, selectedBranch]);

  /* --------------- Actions --------------- */

  const loadBranches = useCallback(
    async (repo?: string, opts?: { force?: boolean }) => {
      const targetRepo = repo ?? repoUrl;
      if (!targetRepo) return [];
      if (!opts?.force && branchesByRepo[targetRepo]) {
        return branchesByRepo[targetRepo];
      }

      const ac = new AbortController();
      begin();
      setError(null);

      try {
        const list = await fetchBranches(targetRepo, provider, {
          signal: ac.signal,
        });
        if (!mountedRef.current) return list;

        setBranchesByRepo((prev) => ({ ...prev, [targetRepo]: list }));

        // If no selected branch or selected branch not present anymore,
        // auto-select the default branch (or first branch).
        const defaultBranch =
          list.find((b) => b.isDefault)?.name ?? list[0]?.name ?? null;
        setSelectedBranch((prevSel) =>
          prevSel && list.some((b) => b.name === prevSel)
            ? prevSel
            : defaultBranch
        );

        return list;
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to fetch branches";
        if (mountedRef.current) setError(msg);
        throw err;
      } finally {
        end();
      }
    },
    [repoUrl, provider, branchesByRepo, fetchBranches]
  );

  const loadBranchContent = useCallback(
    async (branch?: string, opts?: { force?: boolean }) => {
      const b = branch ?? selectedBranch;
      if (!repoUrl || !b) throw new Error("Missing repoUrl or branch");

      const cacheKey = `${repoUrl}#${b}`;
      if (!opts?.force && contentByRepoBranch[cacheKey]) {
        return contentByRepoBranch[cacheKey];
      }

      const ac = new AbortController();
      begin();
      setError(null);

      try {
        const data = await fetchBranchContent(repoUrl, provider, b, {
          signal: ac.signal,
        });
        if (!mountedRef.current) return data;

        setContentByRepoBranch((prev) => ({ ...prev, [cacheKey]: data }));

        // Clear a previously chosen path if it no longer exists in the new content
        if (selectedPath) {
          const exists = findPath(data.items, selectedPath);
          if (!exists) setSelectedPath(null);
        }

        return data;
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to fetch branch content";
        if (mountedRef.current) setError(msg);
        throw err;
      } finally {
        end();
      }
    },
    [
      repoUrl,
      provider,
      selectedBranch,
      contentByRepoBranch,
      fetchBranchContent,
      selectedPath,
    ]
  );

  const importSelectedApiProject = useCallback(
    async (api: ImportApiProjectRequest["api"]) => {
      if (!repoUrl) throw new Error("Repo URL is not set");
      if (!selectedBranch) throw new Error("No branch selected");
      if (!selectedPath) throw new Error("No API project path selected");

      const payload: ImportApiProjectRequest = {
        repoUrl,
        provider,
        branch: selectedBranch,
        path: selectedPath,
        api,
      };

      const ac = new AbortController();
      begin();
      setError(null);

      try {
        const summary = await importApiProject(payload, { signal: ac.signal });
        return summary;
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to import API project";
        if (mountedRef.current) setError(msg);
        throw err;
      } finally {
        end();
      }
    },
    [repoUrl, provider, selectedBranch, selectedPath, importApiProject]
  );

  /* --------------- Memo value --------------- */

  const value = useMemo<GithubAPICreationContextValue>(
    () => ({
      provider,
      repoUrl,
      setProvider,
      setRepoUrl,

      branches,
      selectedBranch,
      setSelectedBranch,
      content,

      selectedPath,
      setSelectedPath,

      loading,
      error,

      loadBranches,
      loadBranchContent,
      importSelectedApiProject,
    }),
    [
      provider,
      repoUrl,
      branches,
      selectedBranch,
      content,
      selectedPath,
      loading,
      error,
      loadBranches,
      loadBranchContent,
      importSelectedApiProject,
    ]
  );

  return (
    <GithubAPICreationContext.Provider value={value}>
      {children}
    </GithubAPICreationContext.Provider>
  );
};

/* ---------------- Hook ---------------- */

export const useGithubAPICreationContext = () => {
  const ctx = useContext(GithubAPICreationContext);
  if (!ctx)
    throw new Error(
      "useGithubAPICreationContext must be used within GithubAPICreationProvider"
    );
  return ctx;
};

/* ---------------- Utilities ---------------- */

function findPath(
  nodes: GitTreeItem[],
  targetPath: string
): GitTreeItem | null {
  for (const n of nodes) {
    if (n.path === targetPath) return n;
    if (n.children?.length) {
      const found = findPath(n.children, targetPath);
      if (found) return found;
    }
  }
  return null;
}
