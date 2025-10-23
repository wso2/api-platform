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
} from "@mui/material";
import CheckCircleRoundedIcon from "@mui/icons-material/CheckCircleRounded";
import { useNavigate, useParams } from "react-router-dom";
import { useOrganization } from "../../context/OrganizationContext";
import { useProjects } from "../../context/ProjectContext";
import { projectSlugFromName } from "../../utils/projectSlug";

import StepTwoApis from "./StepTwoApis";
import StepThreeTest from "./StepThreeTest";
import GatewayCards from "./GatewayCards";
import GatewayForm from "./GatewayForm";
import GatewayList from "./GatewayList";
import OverviewSummary from "./OverviewSummary";

import type { GatewayRecord, GwMode, GwType } from "./types";
import { codeFor } from "./utils";

// If you’re using a custom Button, keep this import.
// Otherwise, swap to MUI: `import { Button } from "@mui/material";`
import { Button } from "../../components/src/components/Button";

type Step = { title: string; subtitle: string };
const STEPS: Step[] = [
  { title: "Add Your Gateway", subtitle: "Lorem Ipsum is simply" },
  { title: "Add Your APIs", subtitle: "Lorem Ipsum is simply" },
  { title: "Test Your APIs", subtitle: "Lorem Ipsum is simply" },
];

export default function GatewayWizard() {
  const navigate = useNavigate();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const { selectedProject } = useProjects();

  const orgHandle = params.orgHandle ?? organization?.handle ?? "";
  const routeProjectSlug = params.projectHandle ?? null;
  const selectedProjectSlug = selectedProject
    ? projectSlugFromName(selectedProject.name, selectedProject.id)
    : null;
  const effectiveProjectSlug = selectedProjectSlug ?? routeProjectSlug;

  const basePath = orgHandle ? `/${orgHandle}` : "";
  const projectBasePath = effectiveProjectSlug
    ? `${basePath}/${effectiveProjectSlug}`
    : null;

  // ---- wizard container state ----
  const [showWizard, setShowWizard] = React.useState(true);
  const [activeStep, setActiveStep] = React.useState<number>(0); // Step 1 initially active

  // ---- state for Step 1/2/3 ----
  const [gwMode, setGwMode] = React.useState<GwMode>("cards");
  const [selectedGateway, setSelectedGateway] = React.useState<GwType | null>(
    null
  );
  const [type, setType] = React.useState<GwType>("hybrid");
  const [editing, setEditing] = React.useState<GatewayRecord | null>(null);
  const [gateways, setGateways] = React.useState<GatewayRecord[]>([]);

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

  // ---- step navigation ----
  const next = () => {
    if (activeStep < STEPS.length - 1) {
      setActiveStep((s) => s + 1);
    } else {
      // Finished → show Summary page
      setShowWizard(false);
      notify("All steps completed 🎉", "success");
    }
  };
  const skip = () => {
    if (activeStep < STEPS.length - 1) setActiveStep((s) => s + 1);
  };

  // ---- Step 1: Gateways ----
  const handleSelectCard = (gw: GwType) => {
    setSelectedGateway(gw);
    setType(gw);
    setEditing(null);
    setGwMode("form");
  };

  const handleCancel = () => {
    setEditing(null);
    setGwMode(gateways.length ? "list" : "cards");
  };

  const handleSubmit = (data: {
    displayName: string;
    name: string;
    host: string;
    description: string;
  }) => {
    if (editing) {
      setGateways((prev) =>
        prev.map((g) => (g.id === editing.id ? { ...g, ...data } : g))
      );
    } else {
      const rec: GatewayRecord = {
        id: String(Date.now()),
        type,
        displayName: data.displayName,
        name: data.name,
        host: data.host,
        description: data.description,
        createdAt: new Date(),
        isActive: false,
      };
      setGateways((prev) => [rec, ...prev]);
    }
    setEditing(null);
    setGwMode("list");
  };
  

  const handleEdit = (g: GatewayRecord) => {
    setType(g.type);
    setEditing(g);
    setGwMode("form");
  };

  const handleDelete = (id: string) => {
    setGateways((prev) => prev.filter((g) => g.id !== id));
  };

  const handleCopy = async (g: GatewayRecord) => {
    try {
      await navigator.clipboard.writeText(codeFor(g.name));
      setGateways((prev) =>
        prev.map((x) => (x.id === g.id ? { ...x, isActive: true } : x))
      );
    } catch {
      // optional: you can notify error here if you want
    }
  };

  // ---- Step 2 helper: mark a gateway active ----
  const markGatewayActive = (id: string) => {
    setGateways((prev) =>
      prev.map((x) => (x.id === id ? { ...x, isActive: true } : x))
    );
  };

  // ---- render ----
  return (
    <Box
      sx={{ minHeight: "100vh", py: 2 }}
      display={"flex"}
      flexDirection={"column"}
      alignItems={"center"}
    >
      {showWizard ? (
        <Container
          maxWidth="lg"
          style={{ display: "flex", flexDirection: "column", alignItems: "center" }}
        >
          {/* TOP: custom stepper */}
          <StepperBar steps={STEPS} activeStep={activeStep} onChange={setActiveStep} />

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
            {/* Step 1: Gateways */}
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
                    type={type}
                    defaults={editing ?? undefined}
                    onCancel={handleCancel}
                    onSubmit={(data) => {
                      const wasEditing = Boolean(editing);
                      handleSubmit(data);
                      notify(
                        wasEditing
                          ? "Gateway updated successfully"
                          : "Successfully added gateway"
                      );
                    }}
                  />
                )}

                {gwMode === "list" && (
                  <GatewayList
                    items={gateways}
                    onAdd={() => {
                      setEditing(null);
                      setGwMode("form");
                    }}
                    onEdit={handleEdit}
                    onDelete={(id) => {
                      handleDelete(id);
                      notify("Gateway deleted", "info");
                    }}
                    onCopy={async (g) => {
                      try {
                        await handleCopy(g);
                        notify("Command copied. Gateway marked Active.");
                      } catch {
                        notify("Unable to copy command", "error");
                      }
                    }}
                  />
                )}
              </>
            )}

            {/* Step 2: APIs */}
            {activeStep === 1 && (
              <StepTwoApis
                gateways={gateways}
                onGoStep1={() => setActiveStep(0)}
                onGatewayActivated={(id) => {
                  markGatewayActive(id);
                  notify("Gateway activated");
                }}
                notify={(msg) => notify(msg)}
                onGoStep3={() => setActiveStep(2)}
              />
            )}

            {/* Step 3: Test */}
            {activeStep === 2 && (
              <StepThreeTest notify={(msg) => notify(msg)} />
            )}

            {/* Bottom-right actions */}
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
      ) : (
        // ---- Summary page (post wizard) ----
        <Container
          maxWidth="lg"
          style={{ display: "flex", flexDirection: "column", alignItems: "center" }}
        >
          <OverviewSummary
            gateways={gateways}
            navigateToGateways={() =>
              navigate(
                projectBasePath ? `${projectBasePath}/gateways` : `${basePath}/gateways`
              )
            }
            navigateToApis={() =>
              navigate(
                projectBasePath ? `${projectBasePath}/apis` : `${basePath}/apis`
              )
            }
          />
        </Container>
      )}
    </Box>
  );
}

/* ======= Stepper ======= */

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
          // pulls each next segment *under* the previous one
          overlap={i === 0 ? 0 : overlap}
          // zIndex: leftmost on top
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

  // flip-headed right arrow for steps 1 & 2, plain for step 3
  const clipPath = last
    ? "none"
    : "polygon(0% 0%, 100% 0%, 92% 0%, 100% 50%, 92% 100%, 100% 100%, 0% 100%, 0% 0%)";

  const basePadX = 24;
  const contentPadLeft = index === 0 ? basePadX : basePadX + overlap;

  // 1px border color by state
  const borderColor = active
    ? "#adb1b1ff"
    : completed
    ? "#e2e8e2ff"
    : theme.palette.grey[300];

  return (
    // WRAPPER: handles overlap, click, and carries the border layer behind
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
      {/* BORDER LAYER (1px bigger so it peeks all around, including arrow head) */}
      <Box
        aria-hidden
        sx={{
          position: "absolute",
          inset: "-1px", // expand 1px on each side
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

      {/* SHAPE LAYER (actual segment content) */}
      <Box
        sx={{
          position: "relative",
          zIndex: 1,
          overflow: "hidden", // prevents next step’s text from bleeding in
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
