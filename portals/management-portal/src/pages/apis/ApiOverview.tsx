import React from "react";
import {
  Alert,
  Box,
  Button,
  Chip,
  CircularProgress,
  Accordion,
  IconButton,
  AccordionSummary,
  AccordionDetails,
  ListItemText,
  Paper,
  Stack,
  Typography,
  Tooltip,
  TextField,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LaunchIcon from "@mui/icons-material/Launch";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import RocketLaunchIcon from "@mui/icons-material/RocketLaunch";

import { ApiProvider, useApisContext } from "../../context/ApiContext";
import type { ApiSummary } from "../../hooks/apis";
import { slugEquals, slugify } from "../../utils/slug";
import theme from "../../theme";

const RESERVED_SLUGS = new Set([
  "overview",
  "gateways",
  "policies",
  "portals",
  "mcp",
  "products",
  "apis",
  "admin",
]);

// --- simple method style map for spec rows (colors mimic screenshot mood)
const METHOD_STYLE: Record<
  string,
  { bg: string; chipColor: "success" | "primary" | "warning" | "error" }
> = {
  POST: { bg: "rgba(16,185,129,0.15)", chipColor: "success" },
  GET: { bg: "rgba(59,130,246,0.12)", chipColor: "primary" },
  PUT: { bg: "rgba(245,158,11,0.15)", chipColor: "warning" },
  DELETE: { bg: "rgba(239,68,68,0.12)", chipColor: "error" },
};

const ApiOverviewContent: React.FC = () => {
  const {
    orgHandle,
    projectHandle,
    apiSlug,
    apiId: legacyApiId,
  } = useParams<{
    orgHandle?: string;
    projectHandle?: string;
    apiSlug?: string;
    apiId?: string;
  }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { apis, fetchApiById, loading } = useApisContext();

  const [apiId, setApiId] = React.useState<string | null>(
    searchParams.get("apiId") ?? legacyApiId ?? null
  );
  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [detailsLoading, setDetailsLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const searchString = searchParams.toString();

  // --- visuals for the spec rows (colors from your reference)
  type MethodKey = "GET" | "POST" | "PUT" | "DELETE" | "PATCH";

  const SPEC_STYLE: Record<
    MethodKey,
    { badgeBg: string; rowBg: string; rowBorder: string }
  > = {
    GET: { badgeBg: "#2F6FE5", rowBg: "#F3F9FF", rowBorder: "#D7E6FF" },
    POST: { badgeBg: "#2FA35C", rowBg: "#F2FBF4", rowBorder: "#D5F1DE" },
    PUT: { badgeBg: "#F39C47", rowBg: "#FFF9ED", rowBorder: "#F6E1BD" },
    DELETE: { badgeBg: "#E05547", rowBg: "#FFF4F4", rowBorder: "#F6CFCC" },
    PATCH: { badgeBg: "#6D28D9", rowBg: "#F4F0FF", rowBorder: "#E0D6FF" },
  };

  const MethodBadge = ({ method }: { method: string }) => {
    const m = (method?.toUpperCase() as MethodKey) || "GET";
    const s = SPEC_STYLE[m] ?? SPEC_STYLE.GET;
    return (
      <Box
        sx={{
          bgcolor: s.badgeBg,
          color: "#fff",
          borderRadius: 1.5,
          px: 2,
          height: 40,
          display: "inline-flex",
          alignItems: "center",
          justifyContent: "center",
          fontWeight: 800,
          letterSpacing: 0.25,
          minWidth: 112,
        }}
      >
        {m}
      </Box>
    );
  };

  React.useEffect(() => {
    if (!apiSlug) return;
    if (RESERVED_SLUGS.has(apiSlug.toLowerCase())) {
      if (projectHandle) {
        navigate("../overview", { replace: true });
      } else if (orgHandle) {
        navigate(`/${orgHandle}/overview`, { replace: true });
      } else {
        navigate("/overview", { replace: true });
      }
    }
  }, [apiSlug, navigate, orgHandle, projectHandle]);

  React.useEffect(() => {
    if (apiId || !apiSlug) return;

    const match = apis.find(
      (item) => slugEquals(item.name, apiSlug) || item.id === apiSlug
    );
    if (match) {
      setApi(match);
      setApiId(match.id);
      const params = new URLSearchParams(searchString);
      params.set("apiId", match.id);
      if (!orgHandle) {
        navigate(`/apis/${match.id}/overview`, { replace: true });
        return;
      }
      const segments: string[] = [orgHandle];
      if (projectHandle) segments.push(projectHandle);
      segments.push("apis", slugify(match.name));
      const base = `/${segments.join("/")}`;
      navigate(`${base}/overview?${params.toString()}`, { replace: true });
    }
  }, [apiId, apiSlug, apis, navigate, searchString, orgHandle, projectHandle]);

  React.useEffect(() => {
    if (!apiId) return;
    setDetailsLoading(true);
    fetchApiById(apiId)
      .then((data) => {
        setApi(data);
        setError(null);
      })
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : "Failed to load API details";
        setError(message);
      })
      .finally(() => setDetailsLoading(false));
  }, [apiId, fetchApiById]);

  const relativeTime = (value?: string | Date | null) => {
    if (!value) return "-";
    const date = value instanceof Date ? value : new Date(value);
    if (Number.isNaN(date.getTime())) return "-";

    const diff = Math.max(0, Date.now() - date.getTime());
    const sec = Math.floor(diff / 1000);
    const min = Math.floor(sec / 60);
    const hr = Math.floor(min / 60);
    const day = Math.floor(hr / 24);
    if (sec < 45) return "just now";
    if (min < 60) return `${min} min ago`;
    if (hr < 24) return `${hr} hr${hr > 1 ? "s" : ""} ago`;
    return `${day} day${day > 1 ? "s" : ""} ago`;
  };

  const ProtocolBadge = ({ label }: { label: string }) => (
    <Chip
      label={label}
      size="small"
      variant="outlined"
      sx={{
        height: 28,
        borderRadius: 1.5,
        px: 1,
        fontWeight: 800,
        letterSpacing: 0.25,
        bgcolor: (t: { palette: { mode: string } }) =>
          t.palette.mode === "dark" ? "rgba(120,150,200,0.12)" : "#EAF2FF",
        borderColor: (t: { palette: { mode: string; divider: any } }) =>
          t.palette.mode === "dark" ? t.palette.divider : "#C7D7F5",
        color: (t: { palette: { text: { primary: any } } }) =>
          t.palette.text.primary,
        "& .MuiChip-label": { px: 1 },
      }}
    />
  );

  const handleBack = React.useCallback(() => {
    if (orgHandle && projectHandle) {
      navigate(`/${orgHandle}/${projectHandle}/apis`);
    } else if (orgHandle) {
      navigate(`/${orgHandle}/apis`);
    } else {
      navigate("/apis");
    }
  }, [navigate, orgHandle, projectHandle]);

  // pick a handy URL to show in “Deployment” panel (first default endpoint if any)
  const firstEndpointUrl =
    api?.backendServices?.find((s) => s.isDefault)?.endpoints?.[0]?.url ??
    api?.backendServices?.[0]?.endpoints?.[0]?.url ??
    "";

  const lifeCycle = (api?.lifeCycleStatus ?? "Published").toString();

  const copyUrl = () => {
    if (!firstEndpointUrl) return;
    navigator.clipboard?.writeText(firstEndpointUrl).catch(() => {});
  };

  if (loading || detailsLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  if (!api) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700} gutterBottom>
          API not found
        </Typography>
        {error && (
          <Typography color="error" sx={{ mb: 2 }}>
            {error}
          </Typography>
        )}
        <Button
          variant="contained"
          startIcon={<ArrowBackIcon />}
          onClick={handleBack}
          sx={{ textTransform: "none" }}
        >
          Back to APIs
        </Button>
      </Box>
    );
  }

  return (
    <Box>
      {/* Top header area */}
      {/* --- Header section redesigned --- */}
      <Stack
        direction={{ xs: "column", lg: "row" }}
        justifyContent="space-between"
        alignItems="flex-start"
        spacing={2}
        sx={{ mb: 2 }}
      >
        {/* Left section: Logo, title, description */}
        <Stack direction="row" spacing={2} alignItems="flex-start">
          <Box
            sx={{
              width: 112,
              height: 112,
              borderRadius: 1,
              backgroundImage: `linear-gradient(135deg,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).light} 0%,
    #059669 55%,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).dark} 100%)`,
              color: "common.white",
              fontWeight: 800,
              fontSize: 28,
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
            }}
          >
            <Typography
              sx={{
                fontSize: 36,
                fontWeight: 600,
                letterSpacing: 0.5,
                lineHeight: 1,
              }}
            >
              {api.name?.slice(0, 2).toUpperCase() || "AP"}
            </Typography>
          </Box>

          <Box maxWidth={700} flex={1}>
            <Stack direction="column" spacing={1} alignItems="flex-start">
              <ProtocolBadge
                label={(api.transport?.[0] ?? "HTTP").toUpperCase()}
              />
              <Typography variant="h6" fontWeight={800}>
                {api.name}
              </Typography>
            </Stack>

            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ maxWidth: 600 }}
            >
              {api.description ||
                "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s."}
            </Typography>
            <Box sx={{ mt: 2, display: "flex", alignItems: "center" }}>
              <Typography variant="subtitle1" sx={{ color: "text.secondary" }}>
                Created
              </Typography>

              {/* subtle clock + time */}
              <Stack
                direction="row"
                spacing={1}
                alignItems="center"
                sx={{ ml: 1 }}
              >
                <Box
                  sx={{
                    width: 24,
                    height: 24,
                    borderRadius: "50%",
                    display: "grid",
                    placeItems: "center",
                    bgcolor: (t) =>
                      t.palette.mode === "dark"
                        ? "rgba(255,255,255,0.08)"
                        : "rgba(2,6,23,0.06)",
                    color: "text.secondary",
                  }}
                >
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden
                  >
                    <path
                      d="M12 7v5l3 2"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                    />
                    <circle
                      cx="12"
                      cy="12"
                      r="9"
                      stroke="currentColor"
                      strokeWidth="2"
                    />
                  </svg>
                </Box>

                <Typography variant="body2" color="text.secondary">
                  {/* {api.createdAt ? relativeTime(new Date(api.createdAt)) : "—"} */}
                  {relativeTime(api.createdAt)}
                </Typography>
              </Stack>
            </Box>
          </Box>
        </Stack>

        {/* Right section: Buttons + status */}
        <Stack spacing={1.25} alignItems="flex-end">
          <Button
            variant="outlined"
            endIcon={<LaunchIcon />}
            sx={{
              textTransform: "none",
              borderColor: "#069668",
              color: "#069668",
              "&:hover": {
                borderColor: "#069668",
                bgcolor: "rgba(6,150,104,0.05)",
              },
            }}
          >
            View on Developer Portal
          </Button>
          <Button
            variant="contained"
            sx={{
              textTransform: "none",
              bgcolor: "#069668",
              "&:hover": { bgcolor: "#047857" },
            }}
          >
            Generate MCP Server
          </Button>

          <Stack spacing={0.25} alignItems="flex-end" sx={{ mt: 0.5 }}>
            <Typography variant="caption" color="text.secondary">
              Lifecycle Status
            </Typography>
            <Chip size="small" label="Published" />
            <Typography variant="caption" color="text.secondary">
              Compliance Summary
            </Typography>
            <Chip
              size="small"
              label="No Policies Applied."
              icon={
                <Box component="span" sx={{ fontSize: 14 }}>
                  ⚠️
                </Box>
              }
            />
          </Stack>
        </Stack>
      </Stack>

      {/* ===== ENV WRAPPER (tabs + Configure + Test/Stop) ===== */}
      <Box
        sx={{
          mt: 3,
          mb: 2,
          borderRadius: 2.5,
          border: `1px solid ${theme.palette.divider}`,
          bgcolor: "background.paper",
          p: 2.25,
        }}
      >
        {/* top row: labels + time + Configure */}
        <Stack direction="row" alignItems="center" spacing={2}>
          <Typography
            variant="subtitle1"
            sx={{ fontWeight: 600, color: "text.primary" }}
          >
            Development
          </Typography>

          {/* <Typography variant="subtitle1" sx={{ color: "text.secondary" }}>
            Deployed
          </Typography> */}

          {/* subtle clock + time */}
          {/* <Stack direction="row" spacing={1} alignItems="center" sx={{ ml: 1 }}>
            <Box
              sx={{
                width: 24,
                height: 24,
                borderRadius: "50%",
                display: "grid",
                placeItems: "center",
                bgcolor: (t) =>
                  t.palette.mode === "dark"
                    ? "rgba(255,255,255,0.08)"
                    : "rgba(2,6,23,0.06)",
                color: "text.secondary",
              }}
            >
              <svg
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                aria-hidden
              >
                <path
                  d="M12 7v5l3 2"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                />
                <circle
                  cx="12"
                  cy="12"
                  r="9"
                  stroke="currentColor"
                  strokeWidth="2"
                />
              </svg>
            </Box>

            <Typography variant="body2" color="text.secondary">
              {relativeTime(api.createdAt)}
            </Typography>
          </Stack> */}
          <Button
            variant="outlined"
            startIcon={<RocketLaunchIcon />}
            sx={{
              textTransform: "none",
              borderColor: "#069668",
              color: "#069668",
              "&:hover": {
                borderColor: "#069668",
                bgcolor: "rgba(6,150,104,0.05)",
              },
            }}
          >
            Deploy
          </Button>
        </Stack>

        {/* thin divider (full width) */}
        <Box sx={{ mt: 2, height: 1, bgcolor: "divider" }} />

        {/* Deployment + URL rows */}
        {/* <Stack direction="row" alignItems="center" spacing={2} sx={{ mt: 3 }}>
          <Typography fontSize={14} sx={{ color: "text.secondary" }}>
            Deployment:
          </Typography>
          <Chip label="Active" sx={{ bgcolor: "#dcfce7", color: "#166534" }} />
        </Stack> */}

        <Box sx={{ mt: 2 }} display={"flex"} flexDirection="row" gap={1}>
          <Typography
            variant="subtitle1"
            sx={{ color: "text.secondary", mb: 0.75 }}
          >
            URL:
          </Typography>
          <TextField
            fullWidth
            size="small"
            value={firstEndpointUrl}
            InputProps={{
              readOnly: true,
              endAdornment: (
                <Tooltip title="Copy URL">
                  <span>
                    <IconButton
                      size="small"
                      onClick={() =>
                        firstEndpointUrl &&
                        navigator.clipboard.writeText(firstEndpointUrl)
                      }
                      disabled={!firstEndpointUrl}
                    >
                      <ContentCopyIcon fontSize="small" />
                    </IconButton>
                  </span>
                </Tooltip>
              ),
            }}
            sx={{
              "& .MuiInputBase-input": { fontSize: 14, minHeight: 30 },
              "& .MuiOutlinedInput-root": {
                bgcolor: (t) =>
                  t.palette.mode === "dark"
                    ? "rgba(255,255,255,0.04)"
                    : "rgba(2,6,23,0.03)",
                borderRadius: 2,
              },
              maxWidth: 1200,
            }}
          />
        </Box>
      </Box>

      {/* API Specifications */}
      <Box
        sx={{
          mt: 1,
          borderRadius: 2,
          border: (t) => `1px solid ${t.palette.divider}`,
          bgcolor: "background.paper",
          p: 2,
        }}
      >
        <Typography
          mb={2}
          variant="subtitle1"
          sx={{ fontWeight: 600, color: "text.primary" }}
        >
          API Specifications
        </Typography>

        {(api.operations ?? []).length === 0 ? (
          <Alert severity="info">No operations defined.</Alert>
        ) : (
          <Stack>
            {(api.operations ?? []).map((op, idx) => {
              const method = (op.request?.method ?? "GET").toUpperCase();
              const path = op.request?.path ?? "-";
              const s = SPEC_STYLE[method as MethodKey] ?? SPEC_STYLE.GET;

              return (
                <Accordion
                  key={`${method}-${path}-${idx}`}
                  elevation={0}
                  disableGutters
                  sx={{
                    border: `1px solid ${s.rowBorder}`,
                    bgcolor: s.rowBg,
                    borderRadius: 2,
                    "&::before": { display: "none" },
                    "&:not(:first-of-type)": { mt: 2 },
                    overflow: "hidden",
                  }}
                >
                  <AccordionSummary
                    expandIcon={<KeyboardArrowDownIcon />}
                    sx={{
                      px: 2,
                      py: 1.25,
                      "& .MuiAccordionSummary-content": {
                        display: "grid",
                        gridTemplateColumns: "auto 1fr auto",
                        alignItems: "center",
                        gap: 8,
                        m: 0,
                      },
                    }}
                  >
                    <MethodBadge method={method} />

                    <Box sx={{ minWidth: 0 }}>
                      <Typography
                        sx={{
                          fontFamily: "monospace",
                          fontSize: 14,
                          lineHeight: 1.2,
                          fontWeight: 700,
                        }}
                      >
                        {path}
                      </Typography>
                      <Typography
                        sx={{
                          color: "text.secondary",
                          fontSize: 12,
                          fontWeight: 500,
                        }}
                        noWrap
                      >
                        {op.description || op.name || "—"}
                      </Typography>
                    </Box>

                    {/* lock icon on the far right */}
                    <LockOutlinedIcon
                      sx={{ color: "text.disabled", fontSize: 20 }}
                    />
                  </AccordionSummary>

                  <AccordionDetails
                    sx={{ bgcolor: "background.default", px: 2, py: 2 }}
                  >
                    <Typography variant="body2" color="text.secondary">
                      {op.description ||
                        "Endpoint documentation and try-it-out consoles will appear here."}
                    </Typography>
                  </AccordionDetails>
                </Accordion>
              );
            })}
          </Stack>
        )}

        {/* View More */}
        <Box sx={{ mt: 2 }}>
          <Button
            startIcon={<LaunchIcon />}
            variant="outlined"
            sx={{ textTransform: "none", borderRadius: 2, px: 1.5, height: 40 }}
          >
            View More
          </Button>
        </Box>
      </Box>
    </Box>
  );
};

const ApiOverview: React.FC = () => (
  <ApiProvider>
    <ApiOverviewContent />
  </ApiProvider>
);

export default ApiOverview;
