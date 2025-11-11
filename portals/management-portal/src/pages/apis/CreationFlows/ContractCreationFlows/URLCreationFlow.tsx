import * as React from "react";
import { Box, Paper, Stack, Typography, Alert, Grid } from "@mui/material";
import { Button } from "../../../../components/src/components/Button";
import { Chip } from "../../../../components/src/components/Chip";
import { TextInput } from "../../../../components/src/components/TextInput";
import CreationMetaData from "../CreationMetaData";

/* ---------- Types (UI-only) ---------- */
type Props = {
  open: boolean;
  onClose: () => void;
  // kept for compatibility with parent, unused in UI-only version
  selectedProjectId?: string;
  createApi?: (..._args: any[]) => Promise<any>;
};

type Step = "url" | "details";

type OpenAPI = {
  paths?: Record<
    string,
    Record<
      string,
      { summary?: string; description?: string; operationId?: string }
    >
  >;
};

/* ---------- helpers for the preview list (UI-only) ---------- */

const METHOD_COLORS: Record<
  string,
  "primary" | "success" | "warning" | "error" | "info" | "secondary"
> = {
  get: "info",
  post: "success",
  put: "warning",
  delete: "error",
  patch: "secondary",
  head: "primary",
  options: "primary",
};

const methodOrder = ["get", "post", "put", "delete", "patch", "head", "options"];

const SwaggerPathList: React.FC<{ doc?: OpenAPI }> = ({ doc }) => {
  if (!doc?.paths) return null;
  const ordered = Object.entries(doc.paths).sort(([a], [b]) => a.localeCompare(b));
  if (!ordered.length) return null;

  return (
    <Box
      sx={{
        border: "1px solid",
        borderColor: "divider",
        borderRadius: 2,
        overflow: "hidden",
        bgcolor: "background.paper",
      }}
    >
      <Box
        sx={{
          px: 2,
          py: 1.25,
          borderBottom: "1px solid",
          borderColor: "divider",
          fontWeight: 700,
        }}
      >
        Fetched OAS Definition
      </Box>
      <Box sx={{ maxHeight: 500, overflow: "auto" }}>
        {ordered.flatMap(([path, ops], i) => {
          const opsOrdered = methodOrder
            .filter((m) => (ops as any)[m])
            .map((m) => [m, (ops as any)[m]!] as const);

          return opsOrdered.map(([method, op], j) => (
            <Box
              key={`${path}-${method}`}
              sx={{
                px: 2,
                py: 1.5,
                display: "grid",
                gridTemplateColumns: "120px 1fr",
                alignItems: "center",
                borderBottom:
                  i === ordered.length - 1 && j === opsOrdered.length - 1
                    ? "none"
                    : "1px solid",
                borderColor: "divider",
              }}
            >
              <Chip
                size="large"
                color={METHOD_COLORS[method] ?? "primary"}
                label={method.toUpperCase()}
                sx={{ fontWeight: 700, width: 72, justifySelf: "start" }}
              />
              <Stack direction="row" spacing={1.5} alignItems="center">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {path}
                </Typography>
                <Typography variant="body2" color="#7c7c7cff" noWrap>
                  {op?.summary || op?.description || ""}
                </Typography>
              </Stack>
            </Box>
          ));
        })}
      </Box>
    </Box>
  );
};

/* ---------- component (UI-only) ---------- */

const URLCreationFlow: React.FC<Props> = ({ open, onClose }) => {
  const [step, setStep] = React.useState<Step>("url");
  const [specUrl, setSpecUrl] = React.useState<string>("");

  // purely for preview demo; in UI-only we can show nothing or a stub
  const [doc] = React.useState<OpenAPI | undefined>(undefined);
  const [error] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (open) {
      setStep("url");
      setSpecUrl("");
    }
  }, [open]);

  if (!open) return null;

  return (
    <Box>
      {step === "url" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
              <Typography variant="subtitle2" sx={{ mb: 1 }}>
                Public Specification URL
              </Typography>
              <TextInput
                label=""
                placeholder="https://example.com/openapi.yaml"
                value={specUrl}
                onChange={(v: string) => setSpecUrl(v)}
                testId=""
                size="medium"
              />

              <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
                <Button
                  variant="text"
                  onClick={() =>
                    setSpecUrl(
                      "https://raw.githubusercontent.com/OAI/OpenAPI-Specification/main/examples/v3.0/petstore.yaml"
                    )
                  }
                >
                  Try with Sample URL
                </Button>
                <Box flex={1} />
                <Button
                  variant="outlined"
                  onClick={() => setStep("details")} // UI-only: just go to next step
                  disabled={!specUrl.trim()}
                >
                  Fetch &amp; Preview
                </Button>
              </Stack>

              {error && (
                <Alert severity="error" sx={{ mt: 2 }}>
                  {error}
                </Alert>
              )}
            </Paper>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            <Paper
              variant="outlined"
              sx={{ p: 3, borderRadius: 2, color: "text.secondary" }}
            >
              <Typography variant="body2">
                Enter a direct URL to an OpenAPI/Swagger document (YAML or JSON).
                We’ll fetch and preview it here.
              </Typography>
            </Paper>
          </Grid>
        </Grid>
      )}

      {step === "details" && (
        <Grid container spacing={2}>
          <Grid size={{ xs: 12, md: 6 }}>
            <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
              <CreationMetaData scope="contract" title="API Details" />

              <Stack
                direction="row"
                spacing={1}
                justifyContent="flex-end"
                sx={{ mt: 3 }}
              >
                <Button variant="outlined" onClick={() => setStep("url")}>
                  Back
                </Button>
                <Button
                  variant="contained"
                  onClick={onClose} // UI-only: close or do nothing
                >
                  Create
                </Button>
              </Stack>
            </Paper>
          </Grid>

          <Grid size={{ xs: 12, md: 6 }}>
            {/* In UI-only mode we have no parsed doc; leave empty or pass a stub */}
            <SwaggerPathList doc={doc} />
          </Grid>
        </Grid>
      )}
    </Box>
  );
};

export default URLCreationFlow;
