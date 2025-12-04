// src/pages/Gateway.tsx
import React from "react";
import { Box, Snackbar, Alert } from "@mui/material";
import { GatewayProvider, useGateways } from "../context/GatewayContext";
import type { Gateway, GatewayType } from "../hooks/gateways";
import type { Components } from "react-markdown";

import GatewayList from "./gateways/GatewayList";
import GatewayForm from "./gateways/GatewayForm";
import GatewayChoose from "./gateways/GatewayChoose";

type Mode = "choose" | "form" | "list";

const slugify = (s: string) =>
  s
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, "")
    .replace(/\s+/g, "-");

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
  `GATEWAY_CONTROLPLANE_TOKEN=${gatewayToken ?? "$GATEWAY_CONTROLPLANE_TOKEN"} docker compose up`;

// --- Minimal bash highlighter used in markdown code block
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
  "curl -sLO https://github.com/wso2/api-platform/releases/download/gateway-v0.0.1/gateway-v0.0.1.zip && \\\n" +
  "unzip gateway-v0.0.1.zip && \\\n" +
  "GATEWAY_CONTROLPLANE_TOKEN=<Token> docker compose --project-directory gateway up\n" +
  "```";

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
  // form state
  const [isCritical, setIsCritical] = React.useState(false);
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

  // auto-generate "name" from displayName
  React.useEffect(() => {
    setName(displayName ? slugify(displayName) : "");
  }, [displayName]);

  React.useEffect(() => {
    editingIdRef.current = editingId;
  }, [editingId]);

  // IMPORTANT: don't flip modes while loading so skeletons can render
  React.useEffect(() => {
    if (gatewaysLoading) return;
    setMode(gateways.length > 0 ? "list" : "choose");
  }, [gateways.length, gatewaysLoading]);

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
    setIsCritical(false);
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
        .catch(() => {});
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
        isCritical,
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

  const handleDelete = async (id: string) => {
    const nextLen = gateways.length - 1;
    try {
      await deleteGatewayByPayload({ gatewayId: id });
      await refreshGateways();
      setSnack({ open: true, msg: "Gateway deleted", severity: "success" });
      if (nextLen <= 0) setMode("choose");
    } catch (err) {
      const msg =
        err instanceof Error ? err.message : "Failed to delete gateway";
      setSnack({ open: true, msg, severity: "error" });
    }
  };

  const handleCopy = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setSnack({
        open: true,
        msg: "Command copied to clipboard",
        severity: "success",
      });
    } catch {
      setSnack({ open: true, msg: "Failed to copy", severity: "error" });
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

  // Choose which view to render; show skeleton list while loading
  const content = gatewaysLoading ? (
    <GatewayList
      loading
      gateways={[]}
      gatewayTokens={gatewayTokens}
      getGatewayStatus={getGatewayStatus}
      gatewayStatuses={gatewayStatuses}
      onAddClick={() => openFormForCreate(type)}
      onEdit={openFormForEdit}
      onDelete={handleDelete}
      onRotateToken={handleRotateToken}
      manualRotatingId={manualRotatingId}
      mdComponentsForCmd={mdComponentsForCmd}
      cmdTextDisplayMd={cmdTextDisplayMd}
      onCopy={handleCopy}
      codeFor={codeFor}
      relativeTime={relativeTime}
    />
  ) : mode === "form" ? (
    <GatewayForm
      type={type}
      editingId={editingId}
      displayName={displayName}
      name={name}
      host={host}
      description={description}
      isCritical={isCritical}
      isSubmitting={isSubmitting}
      onChangeDisplayName={setDisplayName}
      onChangeHost={setHost}
      onChangeDescription={setDescription}
      onChangeIsCritical={setIsCritical}
      onCancel={handleCancel}
      onSubmit={handleAddOrSave}
      onBack={() => setMode(gateways.length ? "list" : "choose")} 
    />
  ) : gateways.length === 0 ? (
    <GatewayChoose onSelectType={handleCardClick} />
  ) : (
    <GatewayList
      loading={false}
      gateways={gateways}
      gatewayTokens={gatewayTokens}
      getGatewayStatus={getGatewayStatus}
      gatewayStatuses={gatewayStatuses}
      onAddClick={() => openFormForCreate(type)}
      onEdit={openFormForEdit}
      onDelete={handleDelete}
      onRotateToken={handleRotateToken}
      manualRotatingId={manualRotatingId}
      mdComponentsForCmd={mdComponentsForCmd}
      cmdTextDisplayMd={cmdTextDisplayMd}
      onCopy={handleCopy}
      codeFor={codeFor}
      relativeTime={relativeTime}
    />
  );

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

