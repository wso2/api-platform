import * as React from "react";
import { Box, Stack, Typography, InputAdornment, Grid } from "@mui/material";
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

type Props = {
  open: boolean;
  onClose: () => void;
  selectedProjectId?: string;
};

type BranchOption = { label: string; value: string };
type Step = "form" | "details";

const GithubCreationFlow: React.FC<Props> = ({ open, onClose }) => {
  const [repoUrl, setRepoUrl] = React.useState("");
  const [branch, setBranch] = React.useState("main");
  const [apiDir, setApiDir] = React.useState("/");
  const [dirModalOpen, setDirModalOpen] = React.useState(false);
  const [step, setStep] = React.useState<Step>("form");

  React.useEffect(() => {
    if (!open) {
      // reset when parent closes
      setRepoUrl("");
      setBranch("main");
      setApiDir("/");
      setDirModalOpen(false);
      setStep("form");
    }
  }, [open]);

  if (!open) return null;

  const showInitial = repoUrl.trim().length === 0;

  const branchOptions: BranchOption[] = React.useMemo(
    () => [
      { label: "main", value: "main" },
      { label: "develop", value: "develop" },
      { label: "release", value: "release" },
    ],
    []
  );

  const selectedBranchOption = React.useMemo<BranchOption | null>(
    () => branchOptions.find((o) => o.value === branch) ?? null,
    [branch, branchOptions]
  );

  const handleBranchChange = (opt: BranchOption | null) => {
    if (opt) setBranch(opt.value);
  };

  const cancelForm = () => {
    // behave like your example "finishAndClose" from other flows:
    setRepoUrl("");
    setBranch("main");
    setApiDir("/");
    setDirModalOpen(false);
    onClose(); // if you prefer to only reset and not close, remove this line
  };

  return (
    <Box>
      {/* ------------ view A: initial card ------------ */}
      {showInitial && step === "form" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Card testId="github-creation-card">
              <CardContent sx={{ p: 3 }}>
                <TextInput
                  label="Public Repository URL"
                  placeholder="https://github.com/org/repo"
                  value={repoUrl}
                  onChange={(v: string) => setRepoUrl(v)}
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
                    onClick={() =>
                      setRepoUrl("https://github.com/wso2/bijira-samples")
                    }
                    endIcon={<ArrowRightLong fontSize="small" />}
                  >
                    Try with Sample URL
                  </Button>

                  <Button
                    variant="outlined"
                    onClick={() => setRepoUrl(repoUrl.trim())}
                    disabled={!repoUrl.trim()}
                  >
                    Continue
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}

      {/* ------------ view B: form (URL/Branch/Dir) ------------ */}
      {!showInitial && step === "form" && (
        <Grid container spacing={2}>
          {/* Row 1: URL | Branch */}
          <Grid size={{ xs: 12, md: 4 }} sx={{ mt: 1 }}>
            <TextInput
              label="Public Repository URL"
              placeholder="https://github.com/org/repo"
              value={repoUrl}
              onChange={(v: string) => setRepoUrl(v)}
              size="medium"
              testId=""
              InputProps={
                {
                  endAdornment: (
                    <InputAdornment position="end">
                      {repoUrl && (
                        <IconButton
                          size="small"
                          onClick={() => setRepoUrl("")}
                          edge="end"
                        >
                          <CloseRoundedIcon fontSize="small" />
                        </IconButton>
                      )}
                    </InputAdornment>
                  ),
                } as any
              }
            />
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
              actions={
                <IconButton
                  aria-label="refresh branches"
                  title="Refresh branches"
                  onClick={() => {
                    // add refresh logic here
                  }}
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
                >
                  Edit
                </Button>
              </Grid>
            </Grid>
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
                disabled={!repoUrl.trim()}
              >
                Next
              </Button>
            </Stack>
          </Grid>
        </Grid>
      )}

      {/* ------------ view C: details (CreationMetaData) ------------ */}
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

      {/* Directory picker modal */}
      <ApiDirectoryModal
        open={dirModalOpen}
        currentPath={apiDir}
        rootLabel={
          repoUrl ? repoUrl.split("/").filter(Boolean).pop() || "repo" : "repo"
        }
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
