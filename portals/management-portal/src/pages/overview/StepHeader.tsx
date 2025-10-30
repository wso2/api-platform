// src/pages/overview/GatewayWizard.tsx
import * as React from "react";
import {
  Box,
  Container,
  Typography,
  Card,
  useTheme,
  Alert,
  Snackbar,
  Chip,
  Tooltip,
  Stack,
} from "@mui/material";
import CheckCircleRoundedIcon from "@mui/icons-material/CheckCircleRounded";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import CachedRoundedIcon from "@mui/icons-material/CachedRounded";
import { useParams } from "react-router-dom";
import { useOrganization } from "../../context/OrganizationContext";
import { useProjects } from "../../context/ProjectContext";
import { projectSlugFromName } from "../../utils/projectSlug";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";

// Step content
import StepTwoApis from "./StepTwoApis";
import StepThreeTest from "./StepThreeTest";
import GatewayCards from "./GatewayCards";
import GatewayForm from "./GatewayForm";

// Context + API types
import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import type { Gateway, GatewayType } from "../../hooks/gateways";

// UI types
import type { GatewayRecord as UIGatewayRecord, GwType } from "./types";

// util
import { codeFor, relativeTime, twoLetters } from "./utils";

// If you’re using a custom Button, keep this import.
import { Button } from "../../components/src/components/Button";

type Step = { title: string; subtitle: string };
const STEPS: Step[] = [
  { title: "Add Your Gateway", subtitle: "Lorem Ipsum is simply" },
  { title: "Add Your APIs", subtitle: "Lorem Ipsum is simply" },
  { title: "Test Your APIs", subtitle: "Lorem Ipsum is simply" },
];

/** Top-level wrapper that provides the Gateway context */
export default function GatewayWizard({ onFinish }: { onFinish?: () => void }) {
  return (
    <GatewayProvider>
      <GatewayWizardContent onFinish={onFinish} />
    </GatewayProvider>
  );
}

/** ---- Adapter: API Gateway -> UI GatewayRecord ---- */
const toUiGateway = (g: Gateway): UIGatewayRecord => ({
  id: g.id,
  type: (g.type ?? "hybrid") as GwType,
  displayName: g.displayName,
  name: g.name,
  host: g.vhost ?? g.host ?? "",
  description: g.description ?? "",
  createdAt: new Date(g.createdAt),
});

/** Small summary card shown right after create */
function CreatedGatewaySummary({
  gateway,
  token,
  onRotate,
  onCopy,
  activeStep,
  onSkip,
  onNext,
  isActive,
}: {
  gateway: Gateway;
  token?: string | null;
  onRotate: () => void;
  // onCopy receives the final text that will be placed on the clipboard
  onCopy: (text: string) => void;
  activeStep: number;
  onSkip: () => void;
  onNext: () => void;
  isActive?: boolean;
}) {
  const ui = toUiGateway(gateway);

  // What we SHOW (placeholder kept as <Token>)
  const cmdTextDisplayMd =
    "```bash\n" +
    "curl -sLO https://github.com/wso2/api-platform/releases/download/v0.1.0-m3/gateway.zip && \\\n" +
    "unzip gateway.zip && \\\n" +
    "GATEWAY_CONTROLPLANE_TOKEN=<Token> docker compose --project-directory gateway up\n" +
    "```";

  // What we COPY (append real token right after <Token>)
  const cmdTextCopy =
    `curl -sLO https://github.com/wso2/api-platform/releases/download/v0.1.0-m3/gateway.zip && \\\n` +
    `unzip gateway.zip && \\\n` +
    `GATEWAY_CONTROLPLANE_TOKEN=${
      token ?? ""
    } docker compose --project-directory gateway up`;

  // --- put this near the top of the file (or in a small utils file) ---
  // 1) Minimal bash token colorizer
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
      if (url) color = "#79a8ff"; // URL
      else if (env) color = "#7dd3fc"; // $GATEWAY_KEY
      else if (cmd) color = cmd === "curl" ? "#b8e78b" : "#f5d67b"; // curl/bash
      else if (flag) color = "#a8acb3"; // -s --name -k
      else if (pipe) color = "#EDEDF0"; // |
      else if (litTest) color = "#f2a36b"; // test

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

  // 2) Use it in react-markdown components (no `inline` typing)
  const mdComponents: Components = {
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
      const isBlock = raw.includes("\n"); // robust inline vs block check

      // avoid passing an incompatible `ref` (causes type clash when multiple React types exist)
      // strip `ref` and `node` which react-markdown may pass
      const { ref, node, ...rest } = props as any;

      if (!isBlock) return <code {...rest}>{raw}</code>;
      return (
        <code {...rest}>
          <BashSyntax text={raw} />
        </code>
      );
    },
  };

  return (
    <>
      <Card
        elevation={0}
        sx={{
          p: 3,
          borderRadius: 1,
          border: "1px solid",
          borderColor: "divider",
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
          {/* Left block */}
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              gap: 2,
              flex: 1,
            }}
          >
            {/* Thumbnail */}
            <Box
              sx={(theme) => ({
                width: 85,
                height: 85,
                borderRadius: 1,
                backgroundImage: `linear-gradient(135deg,
                  ${
                    theme.palette.augmentColor({ color: { main: "#059669" } })
                      .light
                  } 0%,
                  #059669 55%,
                  ${
                    theme.palette.augmentColor({ color: { main: "#059669" } })
                      .dark
                  } 100%)`,
                color: "common.white",
                fontWeight: 800,
                fontSize: 28,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
              })}
              aria-label={`${ui.displayName} thumbnail`}
            >
              {twoLetters(ui.displayName || ui.name)}
            </Box>

            {/* Text */}
            <Box sx={{ flex: 1 }}>
              <Stack direction="row" spacing={1} alignItems="center">
                <Chip
                  label={ui.type === "hybrid" ? "On Premise" : "Cloud"}
                  color="info"
                  variant="outlined"
                  sx={{ borderRadius: 1 }}
                />
              </Stack>
              <Typography variant="h5" fontWeight={800} sx={{ mt: 0.5 }}>
                {ui.displayName}
              </Typography>
              {ui.description && (
                <Typography
                  color="text.secondary"
                  sx={{ mt: 0.2, maxWidth: 900 }}
                >
                  {ui.description}
                </Typography>
              )}
            </Box>
          </Box>
        </Box>

        {/* Meta rows */}
        <Box>
          <Box display={"flex"} gap={4} mt={2} alignItems={"center"}>
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
                {relativeTime(ui.createdAt)}
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
            <Typography sx={{ mt: 0.5 }}>{ui.host || "-"}</Typography>
          </Box>
          <Box display={"flex"} gap={4} mb={1} alignItems={"center"}>
            <Typography color="text.disabled">Replicas:</Typography>
            <Typography sx={{ mt: 0.5 }}>01</Typography>
          </Box>
        </Box>

        <Typography variant="body2" sx={{ mb: 1 }}>
          Run this command locally to start the gateways
        </Typography>

        <Box sx={{ position: "relative" }}>
          {/* Pre to preserve the three lines */}
          <Box sx={{ position: "relative" }}>
            <ReactMarkdown components={mdComponents}>
              {cmdTextDisplayMd}
            </ReactMarkdown>
          </Box>

          <Box
            sx={{
              position: "absolute",
              top: 16,
              right: 2,
              display: "flex",
              gap: 1,
            }}
          >
            {/* <Tooltip title="Rotate token">
              <Button
                variant="text"
                onClick={onRotate}
                size="medium"
                style={{ color: "#fff" }}
                startIcon={<CachedRoundedIcon />}
              />
            </Tooltip> */}

            <Tooltip title={token ? "Copy command" : "Rotate token first"}>
              <span>
                <Button
                  variant="text"
                  onClick={() => onCopy(cmdTextCopy)}
                  size="medium"
                  disabled={!token}
                  style={{ color: "#fff" }}
                  startIcon={<ContentCopyIcon />}
                />
              </span>
            </Tooltip>
          </Box>
        </Box>
      </Card>

      {/* Skip / Next under the card */}
      <Box
        sx={{
          marginTop: 3,
          display: "flex",
          gap: 2,
          alignItems: "center",
          justifyContent: "flex-end",
        }}
      >
        {activeStep < STEPS.length - 1 && (
          <Button
            variant="text"
            onClick={onSkip}
            style={{ color: "#059669", borderColor: "#059669" }}
          >
            Skip
          </Button>
        )}
        <Button
          variant="contained"
          onClick={onNext}
          style={{ backgroundColor: "#059669", borderColor: "#059669" }}
        >
          {activeStep === STEPS.length - 1 ? "Finish" : "Next"}
        </Button>
      </Box>
    </>
  );
}

function GatewayWizardContent({ onFinish }: { onFinish?: () => void }) {
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const { selectedProject } = useProjects();

  const orgHandle = params.orgHandle ?? organization?.handle ?? "";
  const routeProjectSlug = params.projectHandle ?? null;
  const selectedProjectSlug = selectedProject
    ? projectSlugFromName(selectedProject.name, selectedProject.id)
    : null;
  const effectiveProjectSlug = selectedProjectSlug ?? routeProjectSlug;

  // ---- wizard container state ----
  const [activeStep, setActiveStep] = React.useState<number>(0); // Step 1 initially active

  // Step 1 modes: only "cards" | "form" | "created" (no list)
  type GwMode = "cards" | "form" | "created";
  const [gwMode, setGwMode] = React.useState<GwMode>("cards");
  const [selectedGateway, setSelectedGateway] =
    React.useState<GatewayType | null>(null);
  const [type, setType] = React.useState<GatewayType>("hybrid");

  // keep the last created gateway in state (to show its summary)
  const [createdGateway, setCreatedGateway] = React.useState<Gateway | null>(
    null
  );

  // ---- snackbar ----
  const [snack, setSnack] = React.useState<{
    open: boolean;
    msg: string;
    severity?: "success" | "info" | "warning" | "error";
  }>({ open: false, msg: "", severity: "success" });

  const notify = (
    msg: string,
    severity: "success" | "info" | "warning" | "error" = "success"
  ) => setSnack({ open: true, msg, severity });
  const closeSnack = () => setSnack((s) => ({ ...s, open: false }));

  // ---- Gateways context wiring ----
  const {
    createGateway,
    refreshGateways,
    rotateGatewayToken,
    gatewayTokens,
    getGatewayStatus,
    gatewayStatuses,
  } = useGateways();

  // token helpers for the created gateway
  const createdToken = createdGateway
    ? gatewayTokens[createdGateway.id]?.token
    : undefined;

  // ---- step navigation ----
  const next = () => {
    if (activeStep < STEPS.length - 1) {
      setActiveStep((s) => s + 1);
    } else {
      notify("All steps completed 🎉", "success");
      onFinish?.();
    }
  };

  const skip = () => {
    if (activeStep < STEPS.length - 1) setActiveStep((s) => s + 1);
  };

  // ---- Step 1 handlers ----
  const handleSelectCard = (gw: GatewayType) => {
    setSelectedGateway(gw);
    setType(gw);
    setGwMode("form");
  };

  const handleCancel = () => {
    setGwMode(createdGateway ? "created" : "cards");
  };

  const handleSubmit = async (data: {
    displayName: string;
    name: string;
    host: string;
    description: string;
    isCritical: boolean; // <-- NEW
  }) => {
    try {
      const gw = await createGateway({
        displayName: data.displayName,
        name: data.name,
        description: data.description || undefined,
        vhost: data.host || undefined,
        type,
        isCritical: data.isCritical, // <-- NEW: from checkbox
        functionalityType: "regular", // <-- NEW: hardcoded
      });

      await refreshGateways();

      try {
        await rotateGatewayToken(gw.id);
      } catch {
        /* non-fatal */
      }

      setCreatedGateway(gw);
      setGwMode("created");
      notify("Successfully added gateway");
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to add gateway";
      notify(msg, "error");
    }
  };

  const handleRotateCreated = async () => {
    if (!createdGateway) return;
    try {
      await rotateGatewayToken(createdGateway.id);
      notify("Gateway token rotated");
    } catch (e) {
      const msg = e instanceof Error ? e.message : "Failed to rotate token";
      notify(msg, "error");
    }
  };

  const handleCopyCreated = async (text: string) => {
    // text already includes `<Token>` + the real token appended (if any)
    try {
      await navigator.clipboard.writeText(text);
      notify("Command copied.");
    } catch {
      notify("Unable to copy command", "error");
    }
  };

  return (
    <Box
      sx={{ minHeight: "100vh", py: 2 }}
      display={"flex"}
      flexDirection={"column"}
      alignItems={"center"}
    >
      <Container
        maxWidth="lg"
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
        }}
      >
        {/* TOP: custom stepper */}
        <StepperBar
          steps={STEPS}
          activeStep={activeStep}
          onChange={setActiveStep}
        />

        {/* CONTENT CARD */}
        <Card
          elevation={0}
          sx={{
            p: 3,
            borderTopLeftRadius: 0,
            borderTopRightRadius: 0,
            borderBottomLeftRadius: 4,
            borderBottomRightRadius: 4,
            border: "1px solid",
            borderColor: "divider",
            maxWidth: 1010,
            width: "100%",
          }}
        >
          {/* Step 1: Gateways (no list) */}
          {activeStep === 0 && (
            <>
              {gwMode === "cards" && (
                <GatewayCards
                  selected={selectedGateway}
                  onSelect={handleSelectCard}
                />
              )}

              {gwMode === "form" && (
                <GatewayForm
                  type={type as GwType}
                  onCancel={handleCancel}
                  onSubmit={handleSubmit}
                />
              )}

              {gwMode === "created" && createdGateway && (
                <CreatedGatewaySummary
                  gateway={createdGateway}
                  token={createdToken ?? null}
                  onRotate={handleRotateCreated}
                  onCopy={handleCopyCreated}
                  activeStep={activeStep}
                  onSkip={skip}
                  onNext={next}
                  isActive={
                    (typeof getGatewayStatus === "function"
                      ? getGatewayStatus(createdGateway.id)
                      : gatewayStatuses?.[createdGateway.id]
                    )?.isActive === true
                  }
                />
              )}
            </>
          )}

          {/* Step 2: APIs */}
          {activeStep === 1 && (
            <StepTwoApis
              gateways={createdGateway ? [toUiGateway(createdGateway)] : []}
              onGoStep1={() => setActiveStep(0)}
              onGatewayActivated={async () => {
                await handleRotateCreated();
              }}
              notify={(msg) => notify(msg)}
              onGoStep3={() => setActiveStep(2)}
            />
          )}

          {/* Step 3: Test */}
          {activeStep === 2 && <StepThreeTest notify={(msg) => notify(msg)} />}

          {/* Bottom-right actions (hidden on step 0) */}
          {activeStep !== 0 && (
            <Box
              sx={{
                marginTop: 3,
                display: "flex",
                gap: 2,
                alignItems: "center",
                justifyContent: "flex-end",
              }}
            >
              {activeStep < STEPS.length - 1 && (
                <Button
                  variant="text"
                  onClick={skip}
                  style={{ color: "#059669", borderColor: "#059669" }}
                >
                  Skip
                </Button>
              )}
              <Button
                variant="contained"
                onClick={next}
                style={{ backgroundColor: "#059669", borderColor: "#059669" }}
              >
                {activeStep === STEPS.length - 1 ? "Finish" : "Next"}
              </Button>
            </Box>
          )}

          {/* Snackbar */}
          <Snackbar
            open={snack.open}
            autoHideDuration={4000}
            onClose={closeSnack}
            anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
          >
            <Alert
              severity={snack.severity ?? "success"}
              onClose={closeSnack}
              variant="filled"
            >
              {snack.msg}
            </Alert>
          </Snackbar>
        </Card>
      </Container>
    </Box>
  );
}

/* ======= Stepper (with your color tweaks) ======= */

function StepperBar({
  steps,
  activeStep,
  onChange,
}: {
  steps: { title: string; subtitle: string }[];
  activeStep: number;
  onChange: (idx: number) => void;
}) {
  const overlap = 36; // width of the arrow nose

  return (
    <Box sx={{ display: "flex", alignItems: "stretch" }}>
      {steps.map((s, i) => (
        <StepSegment
          key={s.title}
          index={i}
          last={i === steps.length - 1}
          active={i === activeStep}
          completed={i < activeStep}
          onClick={() => onChange(i)}
          overlap={i === 0 ? 0 : overlap}
          zIndex={steps.length - i}
          title={s.title}
          subtitle={s.subtitle}
        />
      ))}
    </Box>
  );
}

function StepSegment({
  index,
  last,
  active,
  completed,
  onClick,
  overlap, // positive number (px)
  zIndex,
  title,
  subtitle,
}: {
  index: number;
  last: boolean;
  active: boolean;
  completed: boolean;
  onClick: () => void;
  overlap: number; // px
  zIndex: number;
  title: string;
  subtitle: string;
}) {
  const theme = useTheme();

  const fill = active
    ? "#eaf7dbff"
    : completed
    ? "#059669"
    : theme.palette.grey[100];

  const textColor = completed ? "#fff" : theme.palette.text.primary;
  const ringColor = completed ? "#fff" : theme.palette.text.primary;

  const clipPath = last
    ? "none"
    : "polygon(0% 0%, 100% 0%, 92% 0%, 100% 50%, 92% 100%, 100% 100%, 0% 100%, 0% 0%)";

  const basePadX = 24;
  const contentPadLeft = index === 0 ? basePadX : basePadX + overlap;

  const borderColor = active
    ? "#adb1b1ff"
    : completed
    ? "#e2e8e2ff"
    : theme.palette.grey[300];

  return (
    <Box
      onClick={onClick}
      sx={{
        position: "relative",
        zIndex,
        cursor: "pointer",
        ml: index === 0 ? 0 : `-${overlap}px`,
        display: "inline-block",
      }}
    >
      {/* BORDER LAYER */}
      <Box
        aria-hidden
        sx={{
          position: "absolute",
          inset: "-1px",
          clipPath,
          bgcolor: borderColor,
          pointerEvents: "none",
          zIndex: 0,
          borderTopLeftRadius: 4,
          borderTopRightRadius: 4,
          borderBottomLeftRadius: 0,
          borderBottomRightRadius: 0,
        }}
      />

      {/* SHAPE LAYER */}
      <Box
        sx={{
          position: "relative",
          zIndex: 1,
          overflow: "hidden",
          bgcolor: fill,
          color: textColor,
          clipPath,
          borderTopLeftRadius: 4,
          borderTopRightRadius: 4,
          borderBottomLeftRadius: 0,
          borderBottomRightRadius: 0,
          minWidth: { xs: 240, md: 360 },
          display: "flex",
          alignItems: "center",
          gap: 2,
          py: 2.25,
          pr: `${basePadX}px`,
          pl: `${contentPadLeft}px`,
          transition: "transform 120ms ease",
        }}
      >
        <Box
          sx={{
            width: 48,
            height: 48,
            borderRadius: "50%",
            border: `1px solid ${ringColor}`,
            display: "grid",
            placeItems: "center",
            flexShrink: 0,
            background: completed ? "rgba(255,255,255,0.12)" : "transparent",
          }}
        >
          {completed ? (
            <CheckCircleRoundedIcon sx={{ color: ringColor }} />
          ) : (
            <Typography fontWeight={600} sx={{ color: textColor }}>
              {(index + 1).toString().padStart(2, "0")}
            </Typography>
          )}
        </Box>

        <Box minWidth={0}>
          <Typography variant="subtitle2" fontWeight={600} noWrap>
            {title}
          </Typography>
          <Typography
            variant="body2"
            noWrap
            sx={{ opacity: active || completed ? 0.95 : 0.7 }}
          >
            {subtitle}
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
