import * as React from "react";
import {
  Alert,
  Box,
  Grid,
  Paper,
  Snackbar,
  Stack,
  Typography,
} from "@mui/material";
import UploadRoundedIcon from "@mui/icons-material/UploadRounded";
import { Button } from "../../../components/src/components/Button";
import { TextInput } from "../../../components/src/components/TextInput";
import yaml from "js-yaml";
import { Chip } from "../../../components/src/components/Chip";
import { IconButton } from "../../../components/src/components/IconButton";
import Delete from "../../../components/src/Icons/generated/Delete";
import ArrowLeftLong from "../../../components/src/Icons/generated/ArrowLeftLong";

type Props = {
  open: boolean;
  selectedProjectId: string;
  createApi: (payload: {
    name: string;
    context: string;
    version: string;
    description?: string;
    projectId: string;
    contract?: string;
    backendServices?: Array<{
      name: string;
      isDefault?: boolean;
      endpoints: Array<{ url: string; description?: string }>;
      retries?: number;
    }>;
    operations?: Array<{
      name: string;
      description?: string;
      request: {
        method: string;
        path: string;
        ["backend-services"]?: Array<{ name: string }>;
      };
    }>;
  }) => Promise<any>;
  onClose: () => void;
};

type Step = "upload" | "details";

type WizardState = {
  name: string;
  context: string;
  version: string;
  description?: string;
  target?: string;
};

type OpenAPI = {
  openapi?: string;
  swagger?: string;
  info?: { title?: string; version?: string; description?: string };
  servers?: { url?: string }[];
  paths?: Record<
    string,
    Record<
      string,
      {
        summary?: string;
        description?: string;
        operationId?: string;
      }
    >
  >;
};

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

function slugify(val: string) {
  return val.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}
function defaultServiceName(apiName: string) {
  const base = apiName?.trim() || "service";
  return `${slugify(base)}-service`;
}
function firstServerUrl(doc?: OpenAPI) {
  const u = doc?.servers?.find((s) => !!s?.url)?.url?.trim();
  return u || "";
}
function deriveContext(doc: OpenAPI, fallbackTitle?: string) {
  const urlStr = doc?.servers?.[0]?.url;
  if (urlStr) {
    try {
      const u = new URL(urlStr, "https://placeholder.local");
      const p = u.pathname || "/";
      const segs = p.split("/").filter(Boolean);
      if (segs.length > 0) return `/${segs[0]}`;
      return "/";
    } catch {
      if (urlStr.startsWith("/")) return urlStr;
    }
  }
  const t = fallbackTitle ? slugify(fallbackTitle) : "api";
  return `/${t}`;
}

const SwaggerPathList: React.FC<{ doc?: OpenAPI }> = ({ doc }) => {
  if (!doc) return null;
  const paths = doc.paths ?? {};
  const ordered = Object.entries(paths).sort(([a], [b]) => a.localeCompare(b));
  if (!ordered.length) return null;

  return (
    <Box sx={{ border: "1px solid", borderColor: "divider", borderRadius: 2, overflow: "hidden", bgcolor: "background.paper" }}>
      <Box sx={{ px: 2, py: 1.25, borderBottom: "1px solid", borderColor: "divider", fontWeight: 700 }}>
        Fetched OAS Definition
      </Box>
      <Box sx={{ maxHeight: 500, overflow: "auto" }}>
        {ordered.map(([path, ops], i) => {
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
                borderBottom: i === ordered.length - 1 && j === opsOrdered.length - 1 ? "none" : "1px solid",
                borderColor: "divider",
              }}
            >
              <Chip size="large" color={METHOD_COLORS[method] ?? "primary"} label={method.toUpperCase()} sx={{ fontWeight: 700, width: 72, justifySelf: "start" }} />
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

const APIContractCreationFlow: React.FC<Props> = ({ open, selectedProjectId, createApi, onClose }) => {
  const [step, setStep] = React.useState<Step>("upload");
  const [rawSpec, setRawSpec] = React.useState<string>("");
  const [doc, setDoc] = React.useState<OpenAPI | undefined>(undefined);
  const [fileName, setFileName] = React.useState<string>("");
  const [error, setError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const [wizardState, setWizardState] = React.useState<WizardState>({
    name: "",
    context: "",
    version: "1.0.0",
    description: "",
    target: "",
  });

  const [snack, setSnack] = React.useState<{ open: boolean; msg: string }>({ open: false, msg: "" });

  // Always-mounted input + stable id/label wiring
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const inputId = React.useId();
  const [fileKey, setFileKey] = React.useState(0);

  React.useEffect(() => {
    if (!open) {
      setStep("upload");
      setRawSpec("");
      setDoc(undefined);
      setFileName("");
      setError(null);
      setWizardState({ name: "", context: "", version: "1.0.0", description: "", target: "" });
      if (fileInputRef.current) fileInputRef.current.value = "";
      setFileKey((k) => k + 1);
    }
  }, [open]);

  const parseSpec = React.useCallback((text: string) => {
    try {
      const trimmed = text.trim();
      if (!trimmed) throw new Error("Empty definition");
      let parsed: any;
      if (trimmed.startsWith("{")) parsed = JSON.parse(trimmed);
      else parsed = yaml.load(text);
      if (!parsed || typeof parsed !== "object") throw new Error("Invalid OpenAPI definition");
      return parsed as OpenAPI;
    } catch (e: any) {
      throw new Error(e?.message || "Invalid OpenAPI definition");
    }
  }, []);

  const autoFill = React.useCallback((d: OpenAPI) => {
    const title = d?.info?.title?.trim() || "";
    const version = d?.info?.version?.trim() || "1.0.0";
    const description = d?.info?.description || "";
    const targetUrl = firstServerUrl(d);

    setWizardState((prev) => ({
      ...prev,
      name: title || prev.name || "Sample API",
      version,
      description,
      context: deriveContext(d, title),
      target: prev.target || targetUrl || "",
    }));
  }, []);

  const handleFiles = React.useCallback(
    async (files: FileList | null) => {
      if (!files || !files[0]) return;
      const f = files[0];
      try {
        setError(null);
        const text = await f.text();
        const parsed = parseSpec(text);
        setRawSpec(text);
        setDoc(parsed);
        setFileName(f.name);
        autoFill(parsed);
      } catch (e: any) {
        setError(e?.message || "OpenAPI Definition validation failed");
      } finally {
        // reset so choosing the same file again will fire onChange
        if (fileInputRef.current) fileInputRef.current.value = "";
        setFileKey((k) => k + 1);
      }
    },
    [autoFill, parseSpec]
  );

  const onDrop = (e: React.DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    handleFiles(e.dataTransfer.files);
    if (fileInputRef.current) fileInputRef.current.value = "";
    setFileKey((k) => k + 1);
  };

  const buildOperationsFromDoc = React.useCallback((d?: OpenAPI, serviceName?: string) => {
    if (!d?.paths)
      return [] as Array<{
        name: string;
        description?: string;
        request: { method: string; path: string; ["backend-services"]: Array<{ name: string }> };
      }>;

    const ops: Array<{
      name: string;
      description?: string;
      request: { method: string; path: string; ["backend-services"]: Array<{ name: string }> };
    }> = [];

    const serviceRef = serviceName ? [{ name: serviceName }] : [];

    const sortedPaths = Object.keys(d.paths).sort((a, b) => a.localeCompare(b));
    for (const path of sortedPaths) {
      const methods = d.paths[path] || {};
      for (const m of methodOrder) {
        const op = (methods as any)[m];
        if (!op) continue;
        const opId = (op.operationId as string | undefined)?.trim();
        const pretty =
          opId ||
          `${m.toUpperCase()} ${path}`
            .replace(/[{}]/g, "")
            .replace(/\s+/g, "_");
        ops.push({
          name: pretty,
          description: op.summary || op.description || undefined,
          request: { method: m.toUpperCase(), path, "backend-services": serviceRef },
        });
      }
    }
    return ops;
  }, []);

  const onCreate = async () => {
    const { name, context, version, description, target } = wizardState;
    if (!name.trim() || !context.trim() || !version.trim()) {
      setError("Please complete all required fields.");
      return;
    }
    const targetUrl = target?.trim();
    let validTarget = true;
    if (targetUrl) {
      try {
        if (/^https?:\/\//i.test(targetUrl)) new URL(targetUrl);
      } catch {
        validTarget = false;
      }
    }
    if (targetUrl && !validTarget) {
      setError("Target must be a valid URL (or leave it empty).");
      return;
    }

    try {
      setCreating(true);
      setError(null);

      const serviceName = defaultServiceName(name.trim());
      const backendServices =
        targetUrl
          ? [
              {
                name: serviceName,
                isDefault: true,
                retries: 2,
                endpoints: [{ url: targetUrl, description: "Primary backend" }],
              },
            ]
          : [];

      const operations = buildOperationsFromDoc(doc, serviceName);

      await createApi({
        name: name.trim(),
        context: context.startsWith("/") ? context.trim() : `/${context.trim()}`,
        version: version.trim(),
        description: description?.trim() || undefined,
        projectId: selectedProjectId,
        contract: rawSpec,
        backendServices,
        operations,
      });

      setSnack({ open: true, msg: `API “${name.trim()}” created successfully.` });
      setTimeout(() => onClose(), 1200);
    } catch (e: any) {
      setError(e?.message || "Failed to create API");
    } finally {
      setCreating(false);
    }
  };

  const handleChange = (patch: Partial<WizardState>) => setWizardState((prev) => ({ ...prev, ...patch }));

  if (!open) return null;

  return (
    <Box>
      {/* Hidden, always-mounted input controlled by labels */}
      <input
        id={inputId}
        ref={fileInputRef}
        key={fileKey}
        type="file"
        accept=".yaml,.yml,.json,.yamal"
        style={{ display: "none" }}
        onChange={(e) => {
          handleFiles(e.target.files);
          e.currentTarget.value = "";
        }}
      />

      {/* Header */}
      <Box mb={1}>
        <Button onClick={onClose} variant="link" startIcon={<ArrowLeftLong fontSize="small" />}>
          Back to List
        </Button>
      </Box>
      <Stack direction="row" alignItems="center" justifyContent="space-between" sx={{ mb: 2 }}>
        <Typography variant="h3" fontWeight={600}>
          Import API Contract
        </Typography>
      </Stack>

      {step === "upload" && (
        <Grid container spacing={2}>
          <Grid  size={{ xs: 12, md: 6 }}>
            <Paper
              component="label"
              htmlFor={inputId}
              variant="outlined"
              sx={{
                p: 4,
                borderStyle: "dashed",
                textAlign: "center",
                minHeight: 280,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                position: "relative",
                backgroundColor: "#f5f5f5ff",
                cursor: "pointer",
              }}
              onDragOver={(e) => e.preventDefault()}
              onDrop={onDrop}
              // No programmatic click — native label→input behavior is more reliable
            >
              {!rawSpec ? (
                <Stack spacing={1} alignItems="center">
                  <Typography variant="h5" fontWeight={600}>
                    Upload API Contract
                  </Typography>
                  <Typography color="#aeacacff">Drag &amp; Drop your files, click, or paste raw spec</Typography>
                  <Button component="label" startIcon={<UploadRoundedIcon />} htmlFor={inputId}>
                    Upload
                  </Button>
                </Stack>
              ) : (
                <Stack spacing={2} alignItems="center">
                  <Typography variant="h6" fontWeight={700}>
                    Uploaded file
                  </Typography>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography color="primary">{fileName}</Typography>
                    <IconButton
                      size="small"
                      color="error"
                      onClick={() => {
                        setRawSpec("");
                        setDoc(undefined);
                        setFileName("");
                        setError(null);
                        if (fileInputRef.current) fileInputRef.current.value = "";
                        setFileKey((k) => k + 1);
                      }}
                    >
                      <Delete fontSize="small" />
                    </IconButton>
                  </Stack>
                </Stack>
              )}
            </Paper>

            <Stack direction="row" spacing={1} sx={{ mt: 2 }}>
              <Button variant="outlined" onClick={onClose} sx={{ textTransform: "none" }}>
                Cancel
              </Button>
              <Button variant="contained" disabled={!rawSpec} onClick={() => setStep("details")} sx={{ textTransform: "none" }}>
                Next
              </Button>
            </Stack>

            {error && (
              <Alert severity="error" sx={{ mt: 2 }}>
                {error}
              </Alert>
            )}
          </Grid>

          <Grid  size={{ xs: 12, md: 6 }}>
            <SwaggerPathList doc={doc} />
          </Grid>
        </Grid>
      )}

      {step === "details" && (
        <Box>
          <Grid container spacing={2}>
            <Grid size={{ xs: 12, md: 6 }}>
              <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
                <Stack spacing={2}>
                  <TextInput label="Name" placeholder="Sample API" value={wizardState.name} onChange={(v: string) => handleChange({ name: v })} testId="" size="medium" />
                  <TextInput
                    label="Target"
                    placeholder="https://api.example.com/v1"
                    value={wizardState.target ?? ""}
                    onChange={(v: string) => handleChange({ target: v })}
                    testId=""
                    size="medium"
                    helperText="Base URL for your backend (used to create a default backend-service)."
                  />
                  <TextInput label="Context" placeholder="/sample" value={wizardState.context} onChange={(v: string) => handleChange({ context: v })} testId="" size="medium" />
                  <TextInput label="Version" placeholder="1.0.0" value={wizardState.version} onChange={(v: string) => handleChange({ version: v })} testId="" size="medium" />
                  <TextInput
                    label="Description"
                    placeholder="Optional description"
                    value={wizardState.description ?? ""}
                    onChange={(v: string) => handleChange({ description: v })}
                    multiline
                    testId=""
                  />
                </Stack>

                <Stack direction="row" spacing={1} justifyContent="flex-end" sx={{ mt: 3 }}>
                  <Button variant="outlined" onClick={() => setStep("upload")} sx={{ textTransform: "none" }}>
                    Back
                  </Button>
                  <Button
                    variant="contained"
                    disabled={!wizardState.name.trim() || !wizardState.context.trim() || !wizardState.version.trim() || creating}
                    onClick={onCreate}
                    sx={{ textTransform: "none" }}
                  >
                    {creating ? "Creating..." : "Create"}
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
              <SwaggerPathList doc={doc} />
            </Grid>
          </Grid>
        </Box>
      )}
      <Snackbar
        open={snack.open}
        autoHideDuration={3000}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
        onClose={() => setSnack((s) => ({ ...s, open: false }))}
      >
        <Alert severity="success" onClose={() => setSnack((s) => ({ ...s, open: false }))}>
          {snack.msg}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default APIContractCreationFlow;
