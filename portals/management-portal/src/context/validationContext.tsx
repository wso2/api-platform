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
  useApiUniquenessValidation,
  type GithubProjectValidationRequest,
  type GithubProjectValidationResponse,
  type ApiNameVersionValidationRequest,
  type ApiUniquenessValidationResponse,
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

  // state (github project validation)
  loading: boolean;
  error: string | null;
  result: GithubProjectValidationResponse | null;

  // actions (github project validation)
  validate: (
    override?: Partial<GithubProjectValidationRequest>
  ) => Promise<GithubProjectValidationResponse>;

  // convenience (github project validation)
  isValid: boolean | null; // null when no result yet
  errors: string[]; // [] when valid or no result

  // name+version uniqueness validation
  nameVersionLoading: boolean;
  nameVersionError: string | null;
  nameVersionResult: ApiUniquenessValidationResponse | null;
  validateNameVersion: (
    payload: ApiNameVersionValidationRequest
  ) => Promise<ApiUniquenessValidationResponse>;
  isNameVersionUnique: boolean | null;

  // identifier uniqueness validation
  identifierLoading: boolean;
  identifierError: string | null;
  identifierResult: ApiUniquenessValidationResponse | null;
  validateIdentifier: (
    identifier: string
  ) => Promise<ApiUniquenessValidationResponse>;
  isIdentifierUnique: boolean | null;

  // reset
  reset: () => void;
};

const Ctx = createContext<GithubProjectValidationContextValue | undefined>(
  undefined
);

type Props = { children: ReactNode };

/** --- Type guards (avoid any / avoid TS2367) --- */
const isGithubValidationErr = (
  r: GithubProjectValidationResponse
): r is Extract<
  GithubProjectValidationResponse,
  { isAPIProjectValid: false }
> => r.isAPIProjectValid === false;

export const GithubProjectValidationProvider: React.FC<Props> = ({
  children,
}) => {
  const { validateGithubApiProject } = useGithubProjectValidation();
  const { validateApiNameVersion, validateApiIdentifier } =
    useApiUniquenessValidation();

  // inputs
  const [repoUrl, setRepoUrl] = useState<string>("");
  const [provider, setProvider] = useState<"github">("github");
  const [branch, setBranch] = useState<string | null>(null);
  const [path, setPath] = useState<string | null>(null);

  // state (github project)
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<GithubProjectValidationResponse | null>(
    null
  );

  // state (name+version)
  const [nameVersionLoading, setNameVersionLoading] = useState(false);
  const [nameVersionError, setNameVersionError] = useState<string | null>(null);
  const [nameVersionResult, setNameVersionResult] =
    useState<ApiUniquenessValidationResponse | null>(null);

  // state (identifier)
  const [identifierLoading, setIdentifierLoading] = useState(false);
  const [identifierError, setIdentifierError] = useState<string | null>(null);
  const [identifierResult, setIdentifierResult] =
    useState<ApiUniquenessValidationResponse | null>(null);

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
        const res = await validateGithubApiProject(effective, {
          signal: ac.signal,
        });
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

  const validateNameVersion = useCallback(
    async (payload: ApiNameVersionValidationRequest) => {
      setNameVersionLoading(true);
      setNameVersionError(null);

      try {
        const res = await validateApiNameVersion(payload);
        if (mountedRef.current) setNameVersionResult(res);
        return res;
      } catch (e) {
        const msg =
          e instanceof Error
            ? e.message
            : "Failed to validate API name & version";
        if (mountedRef.current) {
          setNameVersionError(msg);
          setNameVersionResult(null);
        }
        throw e;
      } finally {
        if (mountedRef.current) setNameVersionLoading(false);
      }
    },
    [validateApiNameVersion]
  );

  const validateIdentifier = useCallback(
    async (identifier: string) => {
      setIdentifierLoading(true);
      setIdentifierError(null);

      try {
        const res = await validateApiIdentifier(identifier);
        if (mountedRef.current) setIdentifierResult(res);
        return res;
      } catch (e) {
        const msg =
          e instanceof Error ? e.message : "Failed to validate API identifier";
        if (mountedRef.current) {
          setIdentifierError(msg);
          setIdentifierResult(null);
        }
        throw e;
      } finally {
        if (mountedRef.current) setIdentifierLoading(false);
      }
    },
    [validateApiIdentifier]
  );

  /** ✅ FIX: no impossible comparisons; use discriminant narrowing */
  const isValid = useMemo(() => {
    if (!result) return null;
    return !isGithubValidationErr(result);
  }, [result]);

  /** ✅ Type-safe errors extraction */
  const errors = useMemo<string[]>(() => {
    if (!result) return [];
    return isGithubValidationErr(result) ? result.errors : [];
  }, [result]);

  const isNameVersionUnique = useMemo(() => {
    if (!nameVersionResult) return null;
    return nameVersionResult.valid;
  }, [nameVersionResult]);

  const isIdentifierUnique = useMemo(() => {
    if (!identifierResult) return null;
    return identifierResult.valid;
  }, [identifierResult]);

  const reset = useCallback(() => {
    pendingRef.current = 0;
    if (!mountedRef.current) return;

    // github project
    setLoading(false);
    setError(null);
    setResult(null);

    // name+version
    setNameVersionLoading(false);
    setNameVersionError(null);
    setNameVersionResult(null);

    // identifier
    setIdentifierLoading(false);
    setIdentifierError(null);
    setIdentifierResult(null);
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

      // github project state
      loading,
      error,
      result,

      // github project actions
      validate,

      // github project convenience
      isValid,
      errors,

      // name+version uniqueness
      nameVersionLoading,
      nameVersionError,
      nameVersionResult,
      validateNameVersion,
      isNameVersionUnique,

      // identifier uniqueness
      identifierLoading,
      identifierError,
      identifierResult,
      validateIdentifier,
      isIdentifierUnique,

      // reset
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
      nameVersionLoading,
      nameVersionError,
      nameVersionResult,
      validateNameVersion,
      isNameVersionUnique,
      identifierLoading,
      identifierError,
      identifierResult,
      validateIdentifier,
      isIdentifierUnique,
      reset,
    ]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
};

export const useGithubProjectValidationContext = () => {
  const ctx = useContext(Ctx);
  if (!ctx) {
    throw new Error(
      "useGithubProjectValidationContext must be used within GithubProjectValidationProvider"
    );
  }
  return ctx;
};
