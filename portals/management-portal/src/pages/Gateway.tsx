// src/pages/Gateway.tsx
import React from "react";
import {
  Box,
  Grid,
  Card,
  CardActionArea,
  CardContent,
  Typography,
  TextField,
  Stack,
  Snackbar,
  Alert,
  Chip,
  Tooltip,
  Tabs,
  Tab,
  FormControlLabel,
  Checkbox,
} from "@mui/material";

import AddIcon from "@mui/icons-material/Add";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import ArrowForwardIosIcon from "@mui/icons-material/ArrowForwardIos";
import OpenInNewOutlinedIcon from "@mui/icons-material/OpenInNewOutlined";
import MenuBookOutlinedIcon from "@mui/icons-material/MenuBookOutlined";
import ShieldOutlinedIcon from "@mui/icons-material/ShieldOutlined";
import BuildOutlinedIcon from "@mui/icons-material/BuildOutlined";
import CachedRoundedIcon from "@mui/icons-material/CachedRounded";

import hybridImg from "../images/hybrid-gateway.svg";
import cloudImg from "../images/cloud-gateway.svg";
import { Button } from "../components/src/components/Button";
import { IconButton } from "../components/src/components/IconButton";
import Edit from "../components/src/Icons/generated/Edit";
import Delete from "../components/src/Icons/generated/Delete";
import Copy from "../components/src/Icons/generated/Copy";
import { GatewayProvider, useGateways } from "../context/GatewayContext";
import type { Gateway, GatewayType } from "../hooks/gateways";
import type { Components } from "react-markdown";
import ReactMarkdown from "react-markdown";

type Mode = "choose" | "form" | "list";

const slugify = (s: string) =>
  s
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-");

const twoLetters = (s: string) => {
  const letters = (s || "").replace(/[^A-Za-z]/g, "");
  if (!letters) return "GW";
  const first = letters[0]?.toUpperCase() ?? "";
  const second = letters[1]?.toLowerCase() ?? "";
  return `${first}${second}`;
};

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

const codeFor = (_gatewayName: string, gatewayToken?: string) =>
  `GATEWAY_CONTROLPLANE_TOKEN=${
    gatewayToken ?? "$GATEWAY_CONTROLPLANE_TOKEN"
  } docker compose up`;

const GatewayContent: React.FC = () => {
  const {
    gateways,
    createGateway,
    updateGateway,
    deleteGateway,
    refreshGateways,
    fetchGatewayById,
    rotateGatewayToken,
    deleteGatewayByPayload,
    gatewayTokens,
    loading: gatewaysLoading,
    getGatewayStatus,
    gatewayStatuses,
  } = useGateways();
  const tokenRequestsRef = React.useRef<Set<string>>(new Set());

  // view mode
  const [mode, setMode] = React.useState<Mode>("choose");

  // which gateway is being edited (null = create)
  const [editingId, setEditingId] = React.useState<string | null>(null);
  const editingIdRef = React.useRef<string | null>(null);

  // selected type (used for form header and when coming from cards)
  const [type, setType] = React.useState<GatewayType>("hybrid");
  // form state (add this)
  const [isCritical, setIsCritical] = React.useState(false);

  // form state
  const [displayName, setDisplayName] = React.useState("");
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [host, setHost] = React.useState("");

  const [isSubmitting, setIsSubmitting] = React.useState(false);
  const [manualRotatingId, setManualRotatingId] = React.useState<string | null>(
    null
  );

  // snackbar
  const [snack, setSnack] = React.useState<{
    open: boolean;
    msg: string;
    severity: "success" | "error";
  }>({ open: false, msg: "", severity: "success" });

  // use case tabs
  const [useCaseTab, setUseCaseTab] = React.useState<"api" | "ai">("api");

  // auto-generate "name" from displayName
  React.useEffect(() => {
    setName(displayName ? slugify(displayName) : "");
  }, [displayName]);

  React.useEffect(() => {
    editingIdRef.current = editingId;
  }, [editingId]);

  React.useEffect(() => {
    if (mode === "choose" && gateways.length > 0) {
      setMode("list");
    } else if (mode === "list" && !gatewaysLoading && gateways.length === 0) {
      setMode("choose");
    }
  }, [gateways.length, gatewaysLoading, mode]);

  React.useEffect(() => {
    tokenRequestsRef.current.forEach((requestedId) => {
      if (gatewayTokens[requestedId]) {
        tokenRequestsRef.current.delete(requestedId);
      }
    });
  }, [gatewayTokens]);

  const resetForm = () => {
    setDisplayName("");
    setDescription("");
    setHost("");
    setName("");
    setEditingId(null);
  };

  const openFormForCreate = (presetType?: GatewayType) => {
    setEditingId(null);
    if (presetType) setType(presetType);
    resetForm();
    setMode("form");
  };

  const openFormForEdit = (g: Gateway) => {
    setEditingId(g.id);
    const nextType = g.type ?? "hybrid";
    setType(nextType);
    setDisplayName(g.displayName);
    setDescription(g.description ?? "");
    setHost(g.vhost ?? g.host ?? "");
    setName(g.name);
    setMode("form");

    if (!g.description || !(g.vhost ?? g.host)) {
      fetchGatewayById(g.id)
        .then((full) => {
          if (editingIdRef.current !== full.id) return;
          setDisplayName(full.displayName);
          setDescription(full.description ?? "");
          setHost(full.vhost ?? full.host ?? "");
          setName(full.name);
          setType(full.type ?? nextType);
        })
        .catch(() => {
          /* error handled via context state */
        });
    }
  };

  const handleCardClick = (gatewayType: GatewayType) => {
    setType(gatewayType);
    openFormForCreate(gatewayType);
  };

  const handleCancel = () => {
    resetForm();
    setMode(gateways.length ? "list" : "choose");
  };

  const handleAddOrSave = async () => {
    if (!displayName.trim() || isSubmitting) return;

    if (editingId) {
      updateGateway(editingId, {
        displayName,
        description,
        name,
        host,
        vhost: host,
        type,
      });
      setSnack({
        open: true,
        msg: "Gateway updated successfully",
        severity: "success",
      });
      resetForm();
      setMode("list");
      return;
    }

    setIsSubmitting(true);
    try {
      await createGateway({
        displayName,
        name,
        description: description || undefined,
        vhost: host || undefined,
        type,
        isCritical, // from checkbox (default false)
        functionalityType: "regular",
      });
      await refreshGateways();
      setSnack({
        open: true,
        msg: "Successfully added gateway",
        severity: "success",
      });
      resetForm();
      setMode("list");
    } catch (err) {
      const msg =
        err instanceof Error ? err.message : "Failed to create gateway";
      setSnack({ open: true, msg, severity: "error" });
    } finally {
      setIsSubmitting(false);
    }
  };

  // small helper component for use-case cards
  const UseCaseCard: React.FC<{
    title: string;
    desc: string;
    onClick?: () => void;
    external?: boolean;
    leftIcon?: React.ReactNode;
  }> = ({ title, desc, onClick, external, leftIcon }) => (
    <Card variant="outlined" sx={{ borderRadius: 2 }}>
      <CardActionArea onClick={onClick}>
        <CardContent
          sx={{
            minHeight: 112,
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 2,
            maxWidth: 360,
          }}
        >
          <Box sx={{ display: "flex", gap: 2 }}>
            <Box
              sx={{
                width: 28,
                height: 28,
                borderRadius: 1,
                // bgcolor: "grey.100",
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                mt: 0.5,
              }}
            >
              {leftIcon ?? <MenuBookOutlinedIcon fontSize="small" />}
            </Box>
            <Box>
              <Typography fontWeight={600}>{title}</Typography>
              <Typography color="text.secondary" variant="body2">
                {desc}
              </Typography>
            </Box>
          </Box>

          <Box sx={{ mt: 0.5 }}>
            {external ? (
              <OpenInNewOutlinedIcon fontSize="small" />
            ) : (
              <ArrowForwardIosIcon fontSize="small" />
            )}
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );

  // delete a gateway by id
  const handleDelete = async (id: string) => {
    const nextLen = gateways.length - 1;
    try {
      await deleteGatewayByPayload({ gatewayId: id }); // ← pass selected id
      await refreshGateways();
      setSnack({
        open: true,
        msg: "Gateway deleted",
        severity: "success",
      });
      if (nextLen <= 0) {
        setMode("choose");
      }
    } catch (err) {
      const msg =
        err instanceof Error ? err.message : "Failed to delete gateway";
      setSnack({ open: true, msg, severity: "error" });
    }
  };

  // Minimal bash highlighter (same as Wizard)
  const BashSyntax = ({ text }: { text: string }) => {
    const pattern =
      /(https?:\/\/\S+)|(\$[A-Z0-9_]+)|\b(curl|bash)\b|(\s--?[a-zA-Z0-9-]+)|(\s\|\s)|\b(test)\b/g;

    const parts: React.ReactNode[] = [];
    let last = 0;
    let m: RegExpExecArray | null;

    while ((m = pattern.exec(text)) !== null) {
      if (m.index > last) parts.push(text.slice(last, m.index));
      const [full, url, env, cmd, flag, pipe, litTest] = m;

      let color = "#EDEDF0";
      if (url) color = "#79a8ff";
      else if (env) color = "#7dd3fc";
      else if (cmd) color = cmd === "curl" ? "#b8e78b" : "#f5d67b";
      else if (flag) color = "#a8acb3";
      else if (pipe) color = "#EDEDF0";
      else if (litTest) color = "#f2a36b";

      parts.push(
        <span key={m.index} style={{ color }}>
          {full}
        </span>
      );
      last = pattern.lastIndex;
    }
    if (last < text.length) parts.push(text.slice(last));
    return <>{parts}</>;
  };

  // Markdown components for the code box
  const mdComponentsForCmd: Components = {
    pre: ({ children }) => (
      <Box
        sx={{
          bgcolor: "#373842",
          color: "#EDEDF0",
          p: 2.5,
          borderRadius: 1,
          fontFamily:
            'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
          fontSize: 14,
          lineHeight: 1.6,
          overflowX: "auto",
          m: 0,
        }}
      >
        <pre style={{ margin: 0, whiteSpace: "pre" }}>{children}</pre>
      </Box>
    ),
    code: (props: any) => {
      const raw = String(props.children ?? "");
      const isBlock = raw.includes("\n");
      const { ref, node, ...rest } = props as any; // strip to avoid ref typing issues
      if (!isBlock) return <code {...rest}>{raw}</code>;
      return (
        <code {...rest}>
          <BashSyntax text={raw} />
        </code>
      );
    },
  };

  const cmdTextDisplayMd =
    "```bash\n" +
    "curl -sLO https://github.com/wso2/api-platform/releases/download/v0.1.0-m3/gateway.zip && \\\n" +
    "unzip gateway.zip && \\\n" +
    "GATEWAY_CONTROLPLANE_TOKEN=<Token> docker compose --project-directory gateway up\n" +
    "```";

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setSnack({
        open: true,
        msg: "Command copied to clipboard",
        severity: "success",
      });
    } catch {
      setSnack({
        open: true,
        msg: "Failed to copy",
        severity: "error",
      });
    }
  };

  const handleRotateToken = React.useCallback(
    async (gatewayId: string) => {
      tokenRequestsRef.current.add(gatewayId);
      setManualRotatingId(gatewayId);
      try {
        await rotateGatewayToken(gatewayId);
        setSnack({
          open: true,
          msg: "Gateway token rotated",
          severity: "success",
        });
      } catch (err) {
        const msg =
          err instanceof Error ? err.message : "Failed to rotate gateway token";
        setSnack({ open: true, msg, severity: "error" });
      } finally {
        setManualRotatingId(null);
        tokenRequestsRef.current.delete(gatewayId);
      }
    },
    [rotateGatewayToken]
  );

  // === Choose view (two cards) when no gateways exist yet ===
  const ChooseView = (
    <>
      <Box textAlign="center" mb={3}>
        <Typography variant="h5" fontWeight={700}>
          Welcome to the My API Project
        </Typography>

        <Typography color="text.secondary" variant="body2">
          Let’s get started with Managing your API Proxies!
        </Typography>
      </Box>

      <Grid container spacing={4} justifyContent="center">
        {/* Hybrid */}
        <Grid>
          <Card
            variant="outlined"
            sx={{
              height: 340,
              borderRadius: 2,
              borderColor: "#069668",
              boxShadow: 3,
              transition: "box-shadow 120ms ease, border-color 120ms ease",
            }}
          >
            <CardActionArea
              sx={{ height: "100%" }}
              onClick={() => handleCardClick("hybrid")}
            >
              <CardContent
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 2,
                }}
              >
                <Box
                  sx={{
                    p: 3,
                    border: "1px dotted",
                    borderColor: "divider",
                    borderRadius: 2,
                  }}
                >
                  <Box
                    component="img"
                    src={hybridImg}
                    alt="Hybrid Gateway"
                    sx={{ width: 140 }}
                  />
                </Box>
                <Typography variant="body1" fontWeight={600}>
                  On Premise Gateway
                </Typography>
                <Typography color="text.secondary" align="center">
                  Let’s get started with creating your Gateways
                </Typography>
              </CardContent>
            </CardActionArea>
          </Card>
        </Grid>

        {/* Cloud */}
        <Grid>
          <Card variant="outlined" sx={{ borderRadius: 2, height: 340 }}>
            <CardActionArea
              sx={{ height: "100%" }}
              onClick={() => handleCardClick("cloud")}
            >
              <CardContent
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 2,
                }}
              >
                <Box
                  sx={{
                    p: 3,
                    border: "1px dotted",
                    borderColor: "divider",
                    borderRadius: 2,
                  }}
                >
                  <Box
                    component="img"
                    src={cloudImg}
                    alt="Cloud Gateway"
                    sx={{ width: 140 }}
                  />
                </Box>
                <Typography variant="h6" fontWeight={700}>
                  Cloud Gateway
                </Typography>
                <Typography color="text.secondary" align="center">
                  Let’s get started with creating your Gateways
                </Typography>
              </CardContent>
            </CardActionArea>
          </Card>
        </Grid>
      </Grid>

      {/* --- Start: Use case section (after gateway cards) --- */}
      <Box
        mt={6}
        display={"flex"}
        flexDirection={"column"}
        alignItems={"center"}
      >
        <Box>
          <Typography variant="body1" fontWeight={600} sx={{ mb: 1 }}>
            Start with a popular use case
          </Typography>

          <Tabs
            value={useCaseTab}
            onChange={(_, v: "api" | "ai") => setUseCaseTab(v)}
            sx={{
              mb: 2,
              "& .MuiTab-root": { textTransform: "none", minHeight: 36 },
            }}
          >
            <Tab label="API Gateway" value="api" />
            <Tab label="AI Gateway" value="ai" />
          </Tabs>
        </Box>

        {useCaseTab === "api" ? (
          <Grid container spacing={2}>
            <Grid>
              <UseCaseCard
                title="Prepare a public API for launch"
                desc="Add authentication and rate limits before launching."
                leftIcon={<BuildOutlinedIcon fontSize="small" />}
                onClick={() => {
                  /* route or open guide */
                }}
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Protect your APIs with OpenID Connect"
                desc="Authorize API clients using your IdP."
                leftIcon={<ShieldOutlinedIcon fontSize="small" />}
                onClick={() => {
                  /* route or open guide */
                }}
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Explore detailed docs"
                desc="Dive deeper into guides and best practices on our developer site."
                external
                onClick={() =>
                  window.open("https://example.dev/docs", "_blank")
                }
              />
            </Grid>
          </Grid>
        ) : (
          <Grid container spacing={2}>
            <Grid>
              <UseCaseCard
                title="Getting started guide"
                desc="Test your AI app with built-in security, metrics, and monitoring."
                external
                onClick={() =>
                  window.open("https://example.dev/ai/get-started", "_blank")
                }
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Explore detailed docs"
                desc="Dive deeper into guides and best practices on our developer site."
                external
                onClick={() =>
                  window.open("https://example.dev/ai/docs", "_blank")
                }
              />
            </Grid>
          </Grid>
        )}
      </Box>
      {/* --- End: Use case section --- */}
    </>
  );

  // === Form view (create / edit) ===
  const FormView = (
    <Box maxWidth={640}>
      <Typography variant="body1" mb={2} fontWeight={600}>
        {editingId
          ? `Edit ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`
          : `Create ${type === "hybrid" ? "On Premise" : "Cloud"} Gateway`}
      </Typography>

      <Stack spacing={2}>
        <TextField
          label="Display name"
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          fullWidth
          autoFocus
          placeholder="e.g., My Production Gateway"
        />

        <TextField
          label="Name"
          value={name}
          fullWidth
          InputProps={{ readOnly: true }}
          helperText="Generated from Display name (read-only)"
        />

        <TextField
          label="Host"
          value={host}
          onChange={(e) => setHost(e.target.value)}
          fullWidth
          placeholder="e.g., gateway.dev.local"
        />

        <TextField
          label="Description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          fullWidth
          multiline
          minRows={3}
          placeholder="Optional description for your gateway"
        />

        <FormControlLabel
          control={
            <Checkbox
              checked={isCritical}
              onChange={(e) => setIsCritical(e.target.checked)}
            />
          }
          label="Mark this as a critical gateway"
        />

        <Stack direction="row" spacing={2} justifyContent="flex-start" mt={1}>
          <Button variant="outlined" onClick={handleCancel}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleAddOrSave}
            disabled={!displayName.trim() || isSubmitting}
            sx={{
              textTransform: "none",
            }}
          >
            {editingId ? "Save" : isSubmitting ? "Adding..." : "Add"}
          </Button>
        </Stack>
      </Stack>
    </Box>
  );

  // === List view ===
  const ListView = (
    <>
      <Box
        mb={2}
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="h5" fontWeight={600}>
          Gateway
        </Typography>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => openFormForCreate(type)}
          sx={{ bgcolor: "primary.dark", textTransform: "none" }}
        >
          Add
        </Button>
      </Box>

      {gateways.map((g) => {
        const latestToken = gatewayTokens[g.id]?.token;
        const hasToken = Boolean(latestToken);
        const status =
          typeof getGatewayStatus === "function"
            ? getGatewayStatus(g.id)
            : gatewayStatuses?.[g.id];

        const isActive = status?.isActive === true;
        const cmdTextCopy =
          `curl -sLO https://github.com/wso2/api-platform/releases/download/v0.1.0-m3/gateway.zip && \\\n` +
          `unzip gateway.zip && \\\n` +
          `GATEWAY_CONTROLPLANE_TOKEN=${latestToken ?? ""} ` +
          `docker compose --project-directory gateway up`;
        const cmd = hasToken
          ? codeFor(g.name, latestToken)
          : "Rotate the gateway token to generate the command.";
        return (
          <Card
            key={g.id}
            elevation={3}
            sx={{
              borderRadius: 1,
              p: 3,
              mb: 3,
            }}
          >
            {/* Header row */}
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 2,
              }}
            >
              {/* Left: avatar + title/desc */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 2,
                  flex: 1,
                }}
              >
                <Box
                  sx={(theme) => ({
                    width: 85,
                    height: 85,
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
                  })}
                  aria-label={`${g.displayName} thumbnail`}
                >
                  {twoLetters(g.displayName || g.name)}
                </Box>

                {/* Title + badge + description */}
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Chip
                      size="small"
                      label={
                        (g.type ?? "hybrid") === "hybrid"
                          ? "On Premise"
                          : "Cloud"
                      }
                      color="info"
                      variant="outlined"
                      style={{ borderRadius: 4 }}
                    />
                  </Stack>
                  <Typography variant="h5" fontWeight={800} sx={{ mt: 0.5 }}>
                    {g.displayName}
                  </Typography>
                  {g.description && (
                    <Typography
                      color="text.secondary"
                      sx={{ mt: 0.2, maxWidth: 900 }}
                    >
                      {g.description}
                    </Typography>
                  )}
                </Box>
              </Box>

              {/* Right: actions */}
              <Stack direction="row" spacing={1}>
                <Tooltip title="Edit">
                  <IconButton
                    color="primary"
                    onClick={() => openFormForEdit(g)}
                  >
                    <Edit />
                  </IconButton>
                </Tooltip>
                <Tooltip title="Delete">
                  <IconButton color="error" onClick={() => handleDelete(g.id)}>
                    <Delete />
                  </IconButton>
                </Tooltip>
              </Stack>
            </Box>

            <Box>
              <Box display={"flex"} gap={4} mt={2}>
                <Typography color="text.disabled">Created:</Typography>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  sx={{ mt: 0.5 }}
                >
                  <AccessTimeIcon
                    fontSize="small"
                    sx={{ color: "text.secondary" }}
                  />
                  <Typography color="text.secondary">
                    {relativeTime(g.createdAt)}
                  </Typography>
                  {isActive ? (
                    <Chip
                      size="small"
                      label="Active"
                      color="success"
                      variant="outlined"
                      style={{ borderRadius: 4 }}
                    />
                  ) : (
                    <Chip
                      size="small"
                      label="Not Active"
                      color="error"
                      variant="outlined"
                      style={{ borderRadius: 4 }}
                    />
                  )}
                </Stack>
              </Box>
              <Box display={"flex"} gap={4} alignItems={"center"}>
                <Typography color="text.disabled">Host:</Typography>
                <Typography sx={{ mt: 0.5 }}>
                  {g.vhost ?? g.host ?? "-"}
                </Typography>
              </Box>
              <Box display={"flex"} gap={4} mb={1} alignItems={"center"}>
                <Typography color="text.disabled">Replicas:</Typography>
                <Typography sx={{ mt: 0.5 }}>02</Typography>
              </Box>
            </Box>

            {/* Command block */}
            {/* Command block */}
            <Box sx={{ mt: 3 }}>
              <Typography variant="body2" sx={{ mb: 1 }}>
                Run This Command locally to start the gateway
              </Typography>

              <Box sx={{ position: "relative" }}>
                <ReactMarkdown components={mdComponentsForCmd}>
                  {cmdTextDisplayMd}
                </ReactMarkdown>

                <Box
                  sx={{
                    position: "absolute",
                    top: 10,
                    right: 6,
                    display: "flex",
                    gap: 1,
                  }}
                >
                  <Tooltip title="Rotate token">
                    <IconButton
                      variant="outlined"
                      onClick={() => handleRotateToken(g.id)}
                      disabled={manualRotatingId === g.id}
                      size="small"
                    >
                      <CachedRoundedIcon
                        style={{ color: "#fff", fill: "#fff" }}
                      />
                    </IconButton>
                  </Tooltip>

                  <Tooltip
                    title={hasToken ? "Copy command" : "Rotate token first"}
                  >
                    <span>
                      <IconButton
                        variant="outlined"
                        onClick={() => handleCopy(cmdTextCopy)}
                        disabled={!hasToken}
                        size="small"
                      >
                        <Copy style={{ color: "#fff", fill: "#fff" }} />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Box>
              </Box>
            </Box>
          </Card>
        );
      })}
    </>
  );

  // Decide which view to render
  const content =
    mode === "form"
      ? FormView
      : gateways.length === 0 && mode === "choose"
      ? ChooseView
      : ListView;

  return (
    <Box>
      {content}

      <Snackbar
        open={snack.open}
        autoHideDuration={4000}
        onClose={() =>
          setSnack((prev) => ({
            ...prev,
            open: false,
          }))
        }
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      >
        <Alert
          severity={snack.severity}
          onClose={() =>
            setSnack((prev) => ({
              ...prev,
              open: false,
            }))
          }
          variant="filled"
        >
          {snack.msg}
        </Alert>
      </Snackbar>
    </Box>
  );
};

const Gateway: React.FC = () => (
  <GatewayProvider>
    <GatewayContent />
  </GatewayProvider>
);

export default Gateway;
