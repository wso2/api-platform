import * as React from "react";
import {
  Box,
  Stack,
  Typography,
  InputAdornment,
  Grid,
  keyframes,
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

// Context
import { useGithubAPICreationContext } from "../../../../context/GithubAPICreationContext";
import Branch from "../../../../components/src/Icons/generated/Branch";

type Props = {
  open: boolean;
  onClose: () => void;
  selectedProjectId?: string;
};

type BranchOption = { label: string; value: string };
type Step = "form" | "details";

const isLikelyGithubRepo = (url: string) =>
  /^https:\/\/github\.com\/[^\/\s]+\/[^\/\s#]+$/i.test(url.trim());

const GithubCreationFlow: React.FC<Props> = ({ open, onClose }) => {
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

  const [apiDir, setApiDir] = React.useState("/");
  const [dirModalOpen, setDirModalOpen] = React.useState(false);
  const [step, setStep] = React.useState<Step>("form");

  // NEW: validation state for API directory
  const [dirError, setDirError] = React.useState<string | null>(null);
  const [isDirValid, setIsDirValid] = React.useState(false);

  // Reset local bits when dialog closes
  React.useEffect(() => {
    if (!open) {
      setApiDir("/");
      setDirModalOpen(false);
      setStep("form");
      setDirError(null);
      setIsDirValid(false);
    }
  }, [open]);

  if (!open) return null;

  const showInitial = (repoUrl ?? "").trim().length === 0;

  // Build select options from fetched branches
  const branchOptions: BranchOption[] = React.useMemo(
    () => branches.map((b) => ({ label: b.name, value: b.name })),
    [branches]
  );

  const spin = keyframes`
  0%   { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
`;

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

  // Auto-select default branch (or first) once branches are available and nothing is selected yet
  React.useEffect(() => {
    if (!branches.length || selectedBranch) return;
    const def =
      branches.find((b) => b.isDefault)?.name ?? branches[0]?.name ?? null;
    if (def) setSelectedBranch(def);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [branches, selectedBranch]);

  // Whenever branch changes:
  //  - fetch branch content
  //  - clear API directory (force re-pick)
  React.useEffect(() => {
    if (!selectedBranch) return;
    setApiDir("/"); // clear any selected directory because branch changed
    setDirError(null);
    setIsDirValid(false);
    loadBranchContent(selectedBranch, { force: true }).catch(() => {});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedBranch]);

  const setSampleRepo = () => {
    const sample = "https://github.com/thivindu/api-platform-demo";
    // changing repo clears state and will auto-fetch branches
    setSelectedBranch(null);
    setApiDir("/");
    setDirModalOpen(false);
    setDirError(null);
    setIsDirValid(false);
    setRepoUrl(sample);
    // immediate fetch (optional; debounced effect will also do it)
    loadBranches(sample, { force: true }).catch(() => {});
  };

  const refreshBranches = () => {
    if (!repoUrl || !isLikelyGithubRepo(repoUrl)) return;
    loadBranches(repoUrl, { force: true }).catch(() => {});
  };

  // Manual change handler for repo URL:
  //  - always clears selected branch, apiDir, modal & dir validation
  const onRepoChange = (v: string) => {
    if (v !== repoUrl) {
      setSelectedBranch(null);
      setApiDir("/");
      setDirModalOpen(false);
      setDirError(null);
      setIsDirValid(false);
    }
    setRepoUrl(v);
  };

  // Branch change (UI): simply set; effects above will fetch+reset dir
  const handleBranchChange = (opt: BranchOption | null) => {
    setSelectedBranch(opt ? opt.value : null);
  };

  const cancelForm = () => {
    setRepoUrl("");
    setSelectedBranch(null);
    setApiDir("/");
    setDirModalOpen(false);
    setStep("form");
    setDirError(null);
    setIsDirValid(false);
  };

  // ===== Directory validation: require a "config.yaml" somewhere under selected folder =====
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
    // No branch content yet, or no directory selected => reset validation state
    if (!content?.items?.length || !apiDir || apiDir === "/") {
      setDirError(null);
      setIsDirValid(false);
      return;
    }
    const target = normalizePath(apiDir); // e.g. "apis/petstore-api"
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
              <Button variant="outlined" onClick={cancelForm}>
                Cancel
              </Button>
              <Button
                variant="contained"
                onClick={() => setStep("details")}
                disabled={
                  !repoUrl.trim() ||
                  !selectedBranch ||
                  !apiDir ||
                  apiDir === "/" ||
                  !isDirValid // NEW: block Next unless config.yaml is present
                }
              >
                Next
              </Button>
            </Stack>
          </Grid>
        </Grid>
      )}

      {/* ------------ Details ------------ */}
      {step === "details" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Card testId={""}>
              <CardContent sx={{ p: 3 }}>
                <CreationMetaData scope="contract" title="API Details" />
                <Stack
                  direction="row"
                  spacing={1}
                  justifyContent="flex-end"
                  sx={{ mt: 3 }}
                >
                  <Button variant="outlined" onClick={() => setStep("form")}>
                    Back
                  </Button>
                  <Button variant="contained" onClick={onClose}>
                    Create
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}

      {/* Directory picker modal — pass fetched items */}
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
          // validation effect will re-run automatically based on apiDir/content
        }}
      />
    </Box>
  );
};

export default GithubCreationFlow;
