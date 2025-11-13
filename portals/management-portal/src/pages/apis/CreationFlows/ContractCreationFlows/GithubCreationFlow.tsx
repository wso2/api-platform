import * as React from "react";
import {
  Box,
  Stack,
  Typography,
  InputAdornment,
  Grid,
  keyframes,
  Alert,
} from "@mui/material";
import CloseRoundedIcon from "@mui/icons-material/CloseRounded";
import { Button } from "../../../../components/src/components/Button";
import { TextInput } from "../../../../components/src/components/TextInput";
import { Card, CardContent } from "../../../../components/src/components/Card";
import { Select as AppSelect } from "../../../../components/src/components/Select";
import ArrowRightLong from "../../../../components/src/Icons/generated/ArrowRightLong";
import ApiDirectoryModal from "./ApiDirectoryModal";
import Refresh from "../../../../components/src/Icons/generated/Refresh";
import { IconButton } from "../../../../components/src/components/IconButton";
import Edit from "../../../../components/src/Icons/generated/Edit";
import CreationMetaData from "../CreationMetaData";

// Contexts
import { useGithubAPICreationContext } from "../../../../context/GithubAPICreationContext";
import { useCreateComponentBuildpackContext } from "../../../../context/CreateComponentBuildpackContext";
import { useGithubProjectValidationContext } from "../../../../context/validationContext";
import Branch from "../../../../components/src/Icons/generated/Branch";
import { ApiOperationsList } from "../../../../components/src/components/Common/ApiOperationsList";
import { useGithubAPICreation } from "../../../../hooks/GithubAPICreation";

/* ---------- Types ---------- */
type Props = {
  open: boolean;
  onClose: () => void;
  selectedProjectId?: string; // must be provided to enable Create
};

type BranchOption = { label: string; value: string };
type Step = "form" | "details";

/* ---------- Utils ---------- */
const isLikelyGithubRepo = (url: string) =>
  /^https:\/\/github\.com\/[^\/\s]+\/[^\/\s#]+$/i.test(url.trim());

const spin = keyframes`
  0%   { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
`;

const GithubCreationFlow: React.FC<Props> = ({
  open,
  onClose,
  selectedProjectId,
}) => {
  const {
    repoUrl,
    setRepoUrl,
    branches,
    selectedBranch,
    setSelectedBranch,
    loadBranches,
    loadBranchContent,
    content,
    loading: ghLoading,
    error: ghError,
  } = useGithubAPICreationContext();

  const {
    validate,
    result: validationResult,
    loading: validating,
    error: validateError,
    reset: resetValidation,
  } = useGithubProjectValidationContext();

  // 👇 We will read meta (name, context, version, target, description) for the POST
  const { contractMeta, setContractMeta } =
    useCreateComponentBuildpackContext();

  // 👇 POST /api/v1/import/api-project
  const { importApiProject } = useGithubAPICreation();

  const [apiDir, setApiDir] = React.useState("/");
  const [dirModalOpen, setDirModalOpen] = React.useState(false);
  const [step, setStep] = React.useState<Step>("form");

  const [dirError, setDirError] = React.useState<string | null>(null);
  const [isDirValid, setIsDirValid] = React.useState(false);

  const [validatedOps, setValidatedOps] = React.useState<
    Array<{
      name?: string;
      description?: string;
      request?: { method?: string; path?: string };
    }>
  >([]);

  // Create flow state
  const [creating, setCreating] = React.useState(false);
  const [createError, setCreateError] = React.useState<string | null>(null);
  const [createSuccessMsg, setCreateSuccessMsg] = React.useState<string | null>(
    null
  );

  // Reset when closed
  React.useEffect(() => {
    if (!open) {
      setApiDir("/");
      setDirModalOpen(false);
      setStep("form");
      setDirError(null);
      setIsDirValid(false);
      setValidatedOps([]);
      resetValidation?.();
      setCreating(false);
      setCreateError(null);
      setCreateSuccessMsg(null);
    }
  }, [open, resetValidation]);

  if (!open) return null;

  const showInitial = (repoUrl ?? "").trim().length === 0;

  // options for branches
  const branchOptions: BranchOption[] = React.useMemo(
    () => branches.map((b) => ({ label: b.name, value: b.name })),
    [branches]
  );

  const selectedBranchOption = React.useMemo<BranchOption | null>(
    () =>
      selectedBranch ? { label: selectedBranch, value: selectedBranch } : null,
    [selectedBranch]
  );

  // Debounced fetch-branches on repoUrl change
  React.useEffect(() => {
    if (!repoUrl || !isLikelyGithubRepo(repoUrl)) return;
    const t = setTimeout(() => {
      loadBranches(repoUrl, { force: true }).catch(() => {});
    }, 500);
    return () => clearTimeout(t);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoUrl]);

  // Auto-select default branch (or first) once branches are available
  React.useEffect(() => {
    if (!branches.length || selectedBranch) return;
    const def =
      branches.find((b) => b.isDefault)?.name ?? branches[0]?.name ?? null;
    if (def) setSelectedBranch(def);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [branches, selectedBranch]);

  // Fetch content when branch changes and clear selections
  React.useEffect(() => {
    if (!selectedBranch) return;
    setApiDir("/");
    setDirError(null);
    setIsDirValid(false);
    setValidatedOps([]);
    resetValidation?.();
    loadBranchContent(selectedBranch, { force: true }).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedBranch]);

  const setSampleRepo = () => {
    const sample = "https://github.com/thivindu/api-platform-demo";
    setSelectedBranch(null);
    setApiDir("/");
    setDirModalOpen(false);
    setDirError(null);
    setIsDirValid(false);
    setValidatedOps([]);
    resetValidation?.();
    setRepoUrl(sample);
    loadBranches(sample, { force: true }).catch(() => {});
  };

  const refreshBranches = () => {
    if (!repoUrl || !isLikelyGithubRepo(repoUrl)) return;
    loadBranches(repoUrl, { force: true }).catch(() => {});
  };

  const onRepoChange = (v: string) => {
    if (v !== repoUrl) {
      setSelectedBranch(null);
      setApiDir("/");
      setDirModalOpen(false);
      setDirError(null);
      setIsDirValid(false);
      setValidatedOps([]);
      resetValidation?.();
    }
    setRepoUrl(v);
  };

  const handleBranchChange = (opt: BranchOption | null) => {
    setSelectedBranch(opt ? opt.value : null);
  };

  // ----- Local directory validation for config.yaml presence -----
  const normalizePath = (p: string) =>
    p.replace(/^\/+/, "").replace(/\/+$/, "");

  const findNodeByPath = React.useCallback(
    (nodes: any[], target: string): any | null => {
      for (const n of nodes) {
        if (n.path === target) return n;
        if (n.children?.length) {
          const hit = findNodeByPath(n.children, target);
          if (hit) return hit;
        }
      }
      return null;
      // eslint-disable-next-line react-hooks/exhaustive-deps
    },
    []
  );

  const nodeHasConfigYaml = React.useCallback((node: any): boolean => {
    if (!node) return false;
    if (node.subPath === "config.yaml") return true;
    if (!node.children?.length) return false;
    for (const c of node.children) {
      if (nodeHasConfigYaml(c)) return true;
    }
    return false;
  }, []);

  React.useEffect(() => {
    if (!content?.items?.length || !apiDir || apiDir === "/") {
      setDirError(null);
      setIsDirValid(false);
      return;
    }
    const target = normalizePath(apiDir);
    const node = findNodeByPath(content.items, target);
    if (!node) {
      setDirError("Selected directory not found in branch content.");
      setIsDirValid(false);
      return;
    }
    const ok = nodeHasConfigYaml(node);
    setIsDirValid(ok);
    setDirError(ok ? null : 'Selected directory must contain a "config.yaml".');
  }, [apiDir, content, findNodeByPath, nodeHasConfigYaml]);

  // ----- Validate on Next, prefill meta, move to details -----
  const onNext = async () => {
    if (
      !repoUrl.trim() ||
      !selectedBranch ||
      !apiDir ||
      apiDir === "/" ||
      !isDirValid
    ) {
      return;
    }

    try {
      const path = normalizePath(apiDir);
      const res = await validate({
        repoUrl,
        provider: "github",
        branch: selectedBranch,
        path,
      });

      const api = (res as any)?.api;
      if (api) {
        const target =
          api["backend-services"]?.[0]?.endpoints?.[0]?.url?.trim() || "";
        // Prefill Meta
        setContractMeta((prev: any) => ({
          ...prev,
          name: api.name || prev?.name || "",
          context: api.context || prev?.context || "",
          version: api.version || prev?.version || "1.0.0",
          description: api.description || prev?.description || "",
          target: target || prev?.target || "",
        }));
        setValidatedOps(Array.isArray(api.operations) ? api.operations : []);
      } else {
        setValidatedOps([]);
      }

      setStep("details");
    } catch {
      setValidatedOps([]);
      setStep("details");
    }
  };

  // ----- Create: POST /api/v1/import/api-project -----
  const onCreate = async () => {
    setCreateError(null);
    setCreateSuccessMsg(null);

    // Guard required fields
    const name = (contractMeta?.name || "").trim();
    const context = (contractMeta?.context || "").trim();
    const version = (contractMeta?.version || "").trim();
    const description = (contractMeta?.description || "").trim() || undefined;
    const target = (contractMeta?.target || "").trim();

    if (!name || !context || !version) {
      setCreateError("Please complete Name, Context, and Version.");
      return;
    }
    if (!repoUrl?.trim() || !selectedBranch) {
      setCreateError("Repository URL and Branch are required.");
      return;
    }
    if (!apiDir || apiDir === "/") {
      setCreateError(
        "Please select the API project directory (contains config.yaml)."
      );
      return;
    }
    if (!selectedProjectId) {
      setCreateError("Project is required (missing projectId).");
      return;
    }

    // build payload
    const payload = {
      repoUrl: repoUrl.trim(),
      provider: "github" as const,
      branch: selectedBranch,
      path: normalizePath(apiDir), // e.g., "apis/test-api"
      api: {
        name,
        displayName: name, // or customize if you prefer a separate display name
        description,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        projectId: selectedProjectId,
        ...(target
          ? {
              ["backend-services"]: [
                {
                  endpoints: [{ url: target }],
                },
              ],
            }
          : {}),
      },
    };

    try {
      setCreating(true);
      await importApiProject(payload);
      setCreateSuccessMsg("API project imported successfully.");
      onClose();
    } catch (e: any) {
      setCreateError(e?.message || "Failed to import API project.");
    } finally {
      setCreating(false);
    }
  };

  const canCreate =
    step === "details" &&
    !validating &&
    !ghLoading &&
    !!selectedProjectId &&
    !!repoUrl?.trim() &&
    !!selectedBranch &&
    !!isDirValid &&
    !!(contractMeta?.name || "").trim() &&
    !!(contractMeta?.context || "").trim() &&
    !!(contractMeta?.version || "").trim();

  return (
    <Box>
      {/* ------------ Initial card ------------ */}
      {showInitial && step === "form" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Card testId="github-creation-card">
              <CardContent sx={{ p: 3 }}>
                <TextInput
                  label="Public Repository URL"
                  placeholder="https://github.com/thivindu/api-platform-demo"
                  value={repoUrl}
                  onChange={onRepoChange}
                  testId=""
                  size="medium"
                />

                <Stack
                  direction="row"
                  justifyContent="space-between"
                  alignItems="center"
                  sx={{ mt: 2 }}
                >
                  <Button
                    variant="text"
                    onClick={setSampleRepo}
                    endIcon={<ArrowRightLong fontSize="small" />}
                  >
                    Try with Sample URL
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}

      {/* ------------ Form (URL/Branch/Dir) ------------ */}
      {!showInitial && step === "form" && (
        <Grid container spacing={2}>
          {/* Row 1: URL | Branch */}
          <Grid size={{ xs: 12, md: 4 }} sx={{ mt: 1 }}>
            <TextInput
              label="Public Repository URL"
              placeholder="https://github.com/org/repo"
              value={repoUrl}
              onChange={onRepoChange}
              size="medium"
              testId="GH-repo-url-input"
              endAdornment={
                <InputAdornment position="end">
                  <IconButton
                    aria-label="refresh branches"
                    title={ghLoading ? "Fetching…" : "Refresh branches"}
                    onClick={ghLoading ? undefined : refreshBranches}
                    size="small"
                    edge="end"
                    disabled={!isLikelyGithubRepo(repoUrl) || ghLoading}
                  >
                    <Refresh
                      fontSize="small"
                      sx={
                        ghLoading
                          ? { animation: `${spin} 0.9s linear infinite` }
                          : undefined
                      }
                    />
                  </IconButton>

                  {repoUrl && (
                    <IconButton
                      size="small"
                      onClick={() => onRepoChange("")}
                      edge="end"
                      title="Clear URL"
                    >
                      <CloseRoundedIcon fontSize="small" />
                    </IconButton>
                  )}
                </InputAdornment>
              }
            />

            {!!ghError && (
              <Typography variant="caption" color="error" lineHeight={0.5}>
                {ghError}
              </Typography>
            )}
            {ghLoading && (
              <Typography variant="caption" color="text.secondary">
                Loading…
              </Typography>
            )}
          </Grid>

          <Grid size={{ xs: 12, md: 2 }}>
            <AppSelect<BranchOption>
              label="Branch"
              labelId="repo-branch"
              name="repo-branch"
              options={branchOptions}
              value={selectedBranchOption}
              onChange={handleBranchChange}
              getOptionLabel={(o) => (typeof o === "string" ? o : o.label)}
              getOptionValue={(o) => (typeof o === "string" ? o : o.value)}
              placeholder="Select branch"
              testId="repo-branch-select"
              size="medium"
              isClearable={false}
              startIcon={<Branch />}
              actions={
                <IconButton
                  aria-label="refresh branches"
                  title="Refresh branches"
                  onClick={refreshBranches}
                  size="small"
                >
                  <Refresh fontSize="small" />
                </IconButton>
              }
            />
          </Grid>

          {/* Row 2: API directory | Edit */}
          <Grid size={{ xs: 12 }}>
            <Grid container spacing={2} alignItems="flex-end">
              <Grid size={{ xs: 12, md: 6 }}>
                <TextInput
                  label="API directory"
                  placeholder="/"
                  value={apiDir}
                  onChange={(v: string) => setApiDir(v)}
                  size="medium"
                  testId=""
                  disabled
                />
              </Grid>
              <Grid size={{ xs: 12, md: 6 }} sx={{ display: "flex" }}>
                <Button
                  variant="link"
                  onClick={() => setDirModalOpen(true)}
                  testId="edit"
                  startIcon={<Edit fontSize="inherit" />}
                  disabled={!selectedBranch || !content?.items?.length}
                >
                  Edit
                </Button>
              </Grid>
            </Grid>
            {!!dirError && (
              <Typography variant="caption" color="error" sx={{ mt: 0.5 }}>
                {dirError}
              </Typography>
            )}
          </Grid>

          {/* Actions row */}
          <Grid size={{ xs: 12 }}>
            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button
                variant="outlined"
                onClick={() => {
                  setRepoUrl("");
                  setSelectedBranch(null);
                  setApiDir("/");
                  setDirModalOpen(false);
                  setStep("form");
                  setDirError(null);
                  setIsDirValid(false);
                  setValidatedOps([]);
                  resetValidation?.();
                }}
              >
                Cancel
              </Button>
              <Button
                variant="contained"
                onClick={onNext}
                disabled={
                  !repoUrl.trim() ||
                  !selectedBranch ||
                  !apiDir ||
                  apiDir === "/" ||
                  !isDirValid ||
                  validating
                }
              >
                {validating ? "Validating…" : "Next"}
              </Button>
            </Stack>
            {!!validateError && (
              <Typography variant="caption" color="error" sx={{ mt: 1 }}>
                {validateError}
              </Typography>
            )}
          </Grid>
        </Grid>
      )}

      {/* ------------ Details ------------ */}
      {step === "details" && (
        <Grid container spacing={2}>
          {/* Validation banner */}
          {validationResult && validationResult.isAPIProjectValid === false && (
            <Grid size={{ xs: 12 }}>
              <Alert severity="error" sx={{ mb: 1.5 }}>
                <Box>
                  <Typography fontWeight={700} mb={0.5}>
                    API project validation failed
                  </Typography>
                  <ul style={{ margin: 0, paddingLeft: 18 }}>
                    {(validationResult.errors || []).map(
                      (e: string, i: number) => (
                        <li key={i}>
                          <Typography variant="body2">{e}</Typography>
                        </li>
                      )
                    )}
                  </ul>
                </Box>
              </Alert>
            </Grid>
          )}

          <Grid size={{ xs: 12, md: 6 }}>
            <Card testId="">
              <CardContent sx={{ p: 3 }}>
                <CreationMetaData scope="contract" title="API Details" />

                {!!createError && (
                  <Alert severity="error" sx={{ mt: 2 }}>
                    {createError}
                  </Alert>
                )}
                {!!createSuccessMsg && (
                  <Alert severity="success" sx={{ mt: 2 }}>
                    {createSuccessMsg}
                  </Alert>
                )}

                <Stack
                  direction="row"
                  spacing={1}
                  justifyContent="flex-end"
                  sx={{ mt: 3 }}
                >
                  <Button
                    variant="outlined"
                    onClick={() => setStep("form")}
                    disabled={creating}
                  >
                    Back
                  </Button>
                  <Button
                    variant="contained"
                    onClick={onCreate}
                    disabled={!canCreate || creating}
                  >
                    {creating ? "Creating..." : "Create"}
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <ApiOperationsList
              title="API Operations (from validation)"
              operations={validatedOps as any}
            />
          </Grid>
        </Grid>
      )}

      {/* Directory picker modal */}
      <ApiDirectoryModal
        open={dirModalOpen}
        currentPath={apiDir}
        rootLabel={
          repoUrl ? repoUrl.split("/").filter(Boolean).pop() || "repo" : "repo"
        }
        items={content?.items ?? []}
        onCancel={() => setDirModalOpen(false)}
        onContinue={(newPath) => {
          setApiDir(newPath);
          setDirModalOpen(false);
        }}
      />
    </Box>
  );
};

export default GithubCreationFlow;
