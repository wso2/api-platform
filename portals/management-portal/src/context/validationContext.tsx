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
  useGithubProjectValidation,
  type GithubProjectValidationRequest,
  type GithubProjectValidationResponse,
} from "../hooks/validation";

/** Keep the name “githubprojectvalidation” semantic in value keys & exports */
type GithubProjectValidationContextValue = {
  // inputs
  repoUrl: string;
  provider: "github";
  branch: string | null;
  path: string | null;

  setRepoUrl: (url: string) => void;
  setProvider: (p: "github") => void;
  setBranch: (b: string | null) => void;
  setPath: (p: string | null) => void;

  // state
  loading: boolean;
  error: string | null;
  result: GithubProjectValidationResponse | null;

  // actions
  validate: (
    override?: Partial<GithubProjectValidationRequest>
  ) => Promise<GithubProjectValidationResponse>;

  // convenience
  isValid: boolean | null; // null when no result yet
  errors: string[]; // [] when valid or no result

  // NEW: allow consumers to clear error/result/loading
  reset: () => void;
};

const Ctx = createContext<GithubProjectValidationContextValue | undefined>(
  undefined
);

type Props = { children: ReactNode };

export const GithubProjectValidationProvider: React.FC<Props> = ({ children }) => {
  const { validateGithubApiProject } = useGithubProjectValidation();

  // inputs
  const [repoUrl, setRepoUrl] = useState<string>("");
  const [provider, setProvider] = useState<"github">("github");
  const [branch, setBranch] = useState<string | null>(null);
  const [path, setPath] = useState<string | null>(null);

  // state
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<GithubProjectValidationResponse | null>(null);

  // lifecycle guards
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

  const validate = useCallback(
    async (override?: Partial<GithubProjectValidationRequest>) => {
      const effective: GithubProjectValidationRequest = {
        repoUrl: override?.repoUrl ?? repoUrl,
        provider: override?.provider ?? provider,
        branch: override?.branch ?? branch ?? "",
        path: override?.path ?? path ?? "",
      };

      if (!effective.repoUrl) throw new Error("repoUrl is required");
      if (!effective.branch) throw new Error("branch is required");
      if (!effective.path) throw new Error("path is required");

      const ac = new AbortController();
      begin();
      setError(null);

      try {
        const res = await validateGithubApiProject(effective, { signal: ac.signal });
        if (mountedRef.current) setResult(res);
        return res;
      } catch (e) {
        const msg =
          e instanceof Error ? e.message : "Failed to validate API project";
        if (mountedRef.current) {
          setError(msg);
          setResult(null);
        }
        throw e;
      } finally {
        end();
      }
    },
    [repoUrl, provider, branch, path, validateGithubApiProject]
  );

  const isValid = useMemo(() => {
    if (!result) return null;
    return (
      !!(result as any).isAPIProjectValid &&
      !!(result as any).isAPIConfigValid &&
      !!(result as any).isAPIDefinitionValid
    );
  }, [result]);

  const errors = useMemo<string[]>(() => {
    if (!result) return [];
    return Array.isArray((result as any).errors) ? (result as any).errors : [];
  }, [result]);

  // NEW: reset helper
  const reset = useCallback(() => {
    pendingRef.current = 0;
    if (!mountedRef.current) return;
    setLoading(false);
    setError(null);
    setResult(null);
  }, []);

  const value = useMemo<GithubProjectValidationContextValue>(
    () => ({
      // inputs
      repoUrl,
      provider,
      branch,
      path,
      setRepoUrl,
      setProvider,
      setBranch,
      setPath,
      // state
      loading,
      error,
      result,
      // actions
      validate,
      // convenience
      isValid,
      errors,
      // new
      reset,
    }),
    [
      repoUrl,
      provider,
      branch,
      path,
      loading,
      error,
      result,
      validate,
      isValid,
      errors,
      reset,
    ]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
};

export const useGithubProjectValidationContext = () => {
  const ctx = useContext(Ctx);
  if (!ctx)
    throw new Error(
      "useGithubProjectValidationContext must be used within GithubProjectValidationProvider"
    );
  return ctx;
};
