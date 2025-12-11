import React from "react";
import {
  Alert,
  Box,
  Chip,
  CircularProgress,
  Accordion,
  IconButton,
  AccordionSummary,
  AccordionDetails,
  Stack,
  Typography,
  Tooltip,
  Collapse,
  Divider,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LaunchIcon from "@mui/icons-material/Launch";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import KeyboardArrowDownIcon from "@mui/icons-material/KeyboardArrowDown";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import RocketLaunchIcon from "@mui/icons-material/RocketLaunch";

import { useApisContext } from "../../context/ApiContext";
import type { ApiSummary, ApiGatewaySummary } from "../../hooks/apis";
import { slugEquals, slugify } from "../../utils/slug";
import theme from "../../theme";
import { Button } from "../../components/src/components/Button";

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

type GatewayListItemProps = {
  gateway: ApiGatewaySummary;
  api: ApiSummary;
};

const GatewayListItem: React.FC<GatewayListItemProps> = ({ gateway, api }) => {
  const [expanded, setExpanded] = React.useState(false);
  const [copiedUrl, setCopiedUrl] = React.useState<string | null>(null);

  const isDeployed = gateway.isDeployed === true;
  const vhost = gateway.vhost || "";
  
  // Construct URLs
  const httpUrl = vhost && api ? `http://${vhost}:8080${api.context}/${api.version}` : null;
  const httpsUrl = vhost && api ? `https://${vhost}:5443${api.context}/${api.version}` : null;
  
  // Get upstream URL (first default backend endpoint)
  const upstreamUrl =
    api?.backendServices?.find((s) => s.isDefault)?.endpoints?.[0]?.url ??
    api?.backendServices?.[0]?.endpoints?.[0]?.url ??
    "";

  const handleCopyUrl = (url: string) => {
    navigator.clipboard?.writeText(url).then(() => {
      setCopiedUrl(url);
      setTimeout(() => setCopiedUrl(null), 2000);
    }).catch(() => {});
  };

  return (
    <Box
      sx={{
        border: (t) => `1px solid ${t.palette.divider}`,
        borderRadius: 2,
        mb: 1.5,
        overflow: "hidden",
        bgcolor: "background.paper",
      }}
    >
      <Box
        sx={{
          p: 2,
          display: "flex",
          alignItems: "center",
          gap: 2,
          cursor: "pointer",
          "&:hover": {
            bgcolor: (t) => t.palette.mode === "dark" ? "rgba(255,255,255,0.03)" : "rgba(0,0,0,0.02)",
          },
        }}
        onClick={() => setExpanded(!expanded)}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 3 }}>
          <Chip
            label={isDeployed ? "Active" : "Inactive"}
            size="small"
            color={isDeployed ? "success" : "default"}
            variant={isDeployed ? "filled" : "outlined"}
            sx={{ minWidth: 85 }}
          />
          
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 250 }}>
            <Typography variant="caption" sx={{ fontWeight: 600, color: "text.secondary" }}>
              Gateway:
            </Typography>
            
            <Typography variant="body1" sx={{ fontWeight: 600 }}>
              {gateway.name}
            </Typography>
          </Box>
        </Box>

        {!expanded && httpsUrl && (
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flex: 1 }}>
            <Typography
              variant="caption"
              sx={{
                fontWeight: 600,
                color: "text.secondary",
                whiteSpace: "nowrap",
              }}
            >
              URL:
            </Typography>
            <Tooltip title={httpsUrl} placement="top">
              <Typography
                variant="body2"
                sx={{
                  flex: 1,
                  fontFamily: "monospace",
                  fontSize: "0.75rem",
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                  whiteSpace: "nowrap",
                  color: "text.secondary",
                }}
                onClick={(e) => e.stopPropagation()}
              >
                {httpsUrl}
              </Typography>
            </Tooltip>
            <Tooltip title={copiedUrl === httpsUrl ? "Copied!" : "Copy HTTPS URL"}>
              <IconButton
                size="small"
                onClick={(e) => {
                  e.stopPropagation();
                  handleCopyUrl(httpsUrl);
                }}
              >
                <ContentCopyIcon sx={{ fontSize: 16 }} />
              </IconButton>
            </Tooltip>
          </Box>
        )}

        {expanded && <Box sx={{ flex: 1 }} />}

        <IconButton
          size="small"
          sx={{
            transform: expanded ? "rotate(180deg)" : "rotate(0deg)",
            transition: "transform 0.2s",
          }}
        >
          <ExpandMoreIcon />
        </IconButton>
      </Box>

      <Collapse in={expanded}>
        <Box sx={{ px: 2, pb: 2, pt: 1, bgcolor: (t) => t.palette.mode === "dark" ? "rgba(255,255,255,0.02)" : "rgba(0,0,0,0.01)" }}>
          <Stack spacing={1.5}>
            {/* Main URLs header + HTTP/HTTPS rows (prefix label + url + copy) */}
            {(httpUrl || httpsUrl) && (
              <Box>
                <Typography variant="caption" sx={{ fontWeight: 600, color: "text.secondary", mb: 0.5, display: "block" }}>
                  Main URLs
                </Typography>

                {httpUrl && (
                  <Box sx={{ mb: 1 }}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1,
                        p: 1.25,
                        bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.03)" : "#F9FAFB"),
                        borderRadius: 1,
                        border: (t) => `1px solid ${t.palette.divider}`,
                      }}
                    >
                      <Typography variant="caption" sx={{ fontWeight: 600, color: "text.secondary", minWidth: 90 }}>
                        HTTP URL
                      </Typography>

                      <Tooltip title={httpUrl} placement="top">
                        <Typography
                          variant="body2"
                          sx={{
                            flex: 1,
                            fontSize: "0.75rem",
                            fontFamily: "monospace",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {httpUrl}
                        </Typography>
                      </Tooltip>

                      <Tooltip title={copiedUrl === httpUrl ? "Copied!" : "Copy HTTP URL"}>
                        <IconButton size="small" onClick={() => handleCopyUrl(httpUrl)}>
                          <ContentCopyIcon sx={{ fontSize: 16 }} />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </Box>
                )}

                {httpsUrl && (
                  <Box sx={{ mb: 1 }}>
                    <Box
                      sx={{
                        display: "flex",
                        alignItems: "center",
                        gap: 1,
                        p: 1.25,
                        bgcolor: (t) => (t.palette.mode === "dark" ? "rgba(255,255,255,0.03)" : "#F9FAFB"),
                        borderRadius: 1,
                        border: (t) => `1px solid ${t.palette.divider}`,
                      }}
                    >
                      <Typography variant="caption" sx={{ fontWeight: 600, color: "text.secondary", minWidth: 90 }}>
                        HTTPS URL
                      </Typography>

                      <Tooltip title={httpsUrl} placement="top">
                        <Typography
                          variant="body2"
                          sx={{
                            flex: 1,
                            fontSize: "0.75rem",
                            fontFamily: "monospace",
                            overflow: "hidden",
                            textOverflow: "ellipsis",
                            whiteSpace: "nowrap",
                          }}
                        >
                          {httpsUrl}
                        </Typography>
                      </Tooltip>

                      <Tooltip title={copiedUrl === httpsUrl ? "Copied!" : "Copy HTTPS URL"}>
                        <IconButton size="small" onClick={() => handleCopyUrl(httpsUrl)}>
                          <ContentCopyIcon sx={{ fontSize: 16 }} />
                        </IconButton>
                      </Tooltip>
                    </Box>
                  </Box>
                )}
              </Box>
            )}

            {upstreamUrl && (
              <>
                <Box sx={{ px: 2 }}>
                  <Divider sx={{ my: 1, borderColor: (t) => t.palette.divider }} />
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ fontWeight: 600, color: "text.secondary", mb: 0.5, display: "block" }}>
                    Upstream URL
                  </Typography>
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                      p: 1.5,
                      bgcolor: (t) => t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "#F9FAFB",
                      borderRadius: 1,
                      border: (t) => `1px solid ${t.palette.divider}`,
                    }}
                  >
                    <Tooltip title={upstreamUrl} placement="top">
                      <Typography
                        variant="body2"
                        sx={{
                          flex: 1,
                          fontSize: "0.75rem",
                          fontFamily: "monospace",
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                        }}
                      >
                        {upstreamUrl}
                      </Typography>
                    </Tooltip>
                    <Tooltip title={copiedUrl === upstreamUrl ? "Copied!" : "Copy Upstream URL"}>
                      <IconButton size="small" onClick={() => handleCopyUrl(upstreamUrl)}>
                        <ContentCopyIcon sx={{ fontSize: 16 }} />
                      </IconButton>
                    </Tooltip>
                  </Box>
                </Box>
              </>
            )}
          </Stack>
        </Box>
      </Collapse>
    </Box>
  );
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
  const { apis, fetchApiById, fetchGatewaysForApi, loading, selectApi } = useApisContext();

  const [apiId, setApiId] = React.useState<string | null>(
    searchParams.get("apiId") ?? legacyApiId ?? null
  );
  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [associatedGateways, setAssociatedGateways] = React.useState<ApiGatewaySummary[]>([]);
  const [gatewaysLoading, setGatewaysLoading] = React.useState(false);
  const [detailsLoading, setDetailsLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const sortedGateways = React.useMemo(() => {
    return [...associatedGateways].sort((a, b) =>
      +(b.isDeployed === true) - +(a.isDeployed === true)
    );
  }, [associatedGateways]);

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
      selectApi(match, { slug: slugify(match.name) });
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
        selectApi(data, { slug: apiSlug ?? slugify(data.name) });
      })
      .catch((err) => {
        const message =
          err instanceof Error ? err.message : "Failed to load API details";
        setError(message);
      })
      .finally(() => setDetailsLoading(false));
  }, [apiId, apiSlug, fetchApiById, selectApi]);

  React.useEffect(() => {
    if (!apiId) return;
    setGatewaysLoading(true);
    fetchGatewaysForApi(apiId)
      .then((gateways) => {
        setAssociatedGateways(gateways);
      })
      .catch((err) => {
        console.error("Failed to load gateways:", err);
        setAssociatedGateways([]);
      })
      .finally(() => setGatewaysLoading(false));
  }, [apiId, fetchGatewaysForApi]);

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
              {api.displayName || api.name}
            </Typography>
          </Box>

          <Box maxWidth={700} flex={1}>
            <Stack direction="column" spacing={1} alignItems="flex-start">
              <ProtocolBadge
                label={(api.transport?.[0] ?? "HTTP").toUpperCase()}
              />
              <Typography variant="h3" fontWeight={800}>
                {api.displayName}
              </Typography>
            </Stack>

            <Typography
              variant="body2"
              color="#636262ff"
              sx={{ maxWidth: 600 }}
            >
              {api.description}
            </Typography>
            <Box sx={{ mt: 2, display: "flex", alignItems: "center" }}>
              <Typography variant="body2" sx={{ color: "#636262ff" }}>
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

                <Typography variant="body2" color="#636262ff">
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
            variant="contained"
            onClick={() => {
              // Navigate to publish screen
              const base =
                orgHandle && projectHandle
                  ? `/${orgHandle}/${projectHandle}/apis/${api.id}/publish`
                  : orgHandle
                  ? `/${orgHandle}/apis/${api.id}/publish`
                  : `/apis/${api.id}/publish`;
              navigate(`${base}?apiId=${api.id}`);
            }}
            sx={{
              textTransform: "none",
              bgcolor: "#069668",
              color: "white",
              "&:hover": {
                bgcolor: "#047857",
              },
            }}
          >
            Add to DevPortal
          </Button>

          <Stack spacing={0.25} alignItems="flex-end" sx={{ mt: 0.5 }}>
            {/* <Typography variant="caption" color="text.secondary">
              Lifecycle Status
            </Typography>
            <Chip size="small" label="Published" /> */}
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
        <Stack direction="row" alignItems="center" justifyContent="space-between">
          <Typography
            variant="subtitle1"
            sx={{ fontWeight: 600, color: "text.primary" }}
          >
            Deployments
          </Typography>

          <Button
            variant="outlined"
            startIcon={<RocketLaunchIcon />}
            onClick={() => {
              // open the "API Proxies" submenu
              window.dispatchEvent(
                new CustomEvent("open-submenu", { detail: { group: "apis" } })
              );

              // build the correct base path (org-only or org+project)
              const base =
                orgHandle && projectHandle
                  ? `/${orgHandle}/${projectHandle}/apis/develop`
                  : orgHandle
                  ? `/${orgHandle}/apis/develop`
                  : `/apis/develop`;

              const params = new URLSearchParams();
              params.set("apiId", api.id);
              params.set("apiSlug", slugify(api.name));

              navigate(`${base}?${params.toString()}`);
            }}
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

        <Box sx={{ mt: 2, mb: 2, height: 1, bgcolor: "divider" }} />

        {gatewaysLoading ? (
          <Box display="flex" alignItems="center" justifyContent="center" py={4}>
            <CircularProgress size={24} />
          </Box>
        ) : associatedGateways.length === 0 ? (
          <Box py={3} textAlign="center">
            <Typography variant="body2" color="text.secondary">
              No gateways associated with this API yet.
            </Typography>
            <Typography variant="caption" color="text.secondary">
              Use the Deploy button to add gateways.
            </Typography>
          </Box>
        ) : (
          <Box>
            {sortedGateways.map((gw) => (
              <GatewayListItem key={gw.id} gateway={gw} api={api} />
            ))}
          </Box>
        )}
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
  <ApiOverviewContent />
);

export default ApiOverview;
