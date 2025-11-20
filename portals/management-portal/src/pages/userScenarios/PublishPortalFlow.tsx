import * as React from "react";
import {
  Box,
  Container,
  Typography,
  Grid,
  Paper,
  Stack,
  Alert,
  CircularProgress,
  useTheme,
} from "@mui/material";
import CheckCircleRoundedIcon from "@mui/icons-material/CheckCircleRounded";
import { TextInput } from "../../components/src/components/TextInput";
import { Button } from "../../components/src/components/Button";
import { ApiOperationsList } from "../../components/src/components/Common/ApiOperationsList";
import CreationMetaData from "../apis/CreationFlows/CreationMetaData";
import WizardPortalCard from "../portals/WizardPortalCard";
import {
  useCreateComponentBuildpackContext,
  CreateComponentBuildpackProvider,
} from "../../context/CreateComponentBuildpackContext";
import {
  useOpenApiValidation,
  type OpenApiValidationResponse,
} from "../../hooks/validation";
import { useDevPortals } from "../../context/DevPortalContext";
import { PORTAL_CONSTANTS } from "../../constants/portal";
import BijiraDPLogo from "../BijiraDPLogo.png";
import type { Portal } from "../../hooks/devportals";

type Step = { title: string; subtitle: string };
const STEPS: Step[] = [
  {
    title: "Import API Definition",
    subtitle: "Validate your OpenAPI spec & select portal",
  },
  { title: "Configure API Details", subtitle: "Set API metadata and publish" },
];

function slugify(val: string) {
  return val
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
}

function defaultServiceName(apiName: string) {
  const base = apiName?.trim() || "service";
  return `${slugify(base)}-service`;
}

function firstServerUrl(api: any) {
  const services = api?.["backend-services"] || [];
  const endpoint = services[0]?.endpoints?.[0]?.url;
  return endpoint?.trim() || "";
}

function deriveContext(api: any) {
  return api?.context || "/api";
}

function mapOperations(
  operations: any[],
  options?: { serviceName?: string; withFallbackName?: boolean },
) {
  if (!Array.isArray(operations)) return [];

  return operations.map((op: any) => ({
    name: options?.withFallbackName
      ? op.name || op.request?.path || "Unknown"
      : op.name,
    description: op.description,
    request: {
      method: op.request?.method || "GET",
      path: op.request?.path || "/",
      ...(options?.serviceName && {
        ["backend-services"]: [{ name: options.serviceName }],
      }),
    },
  }));
}

function PublishPortalFlowContent({ onFinish }: { onFinish?: () => void }) {
  const [activeStep, setActiveStep] = React.useState(0);
  const [specUrl, setSpecUrl] = React.useState<string>("");
  const [validationResult, setValidationResult] =
    React.useState<OpenApiValidationResponse | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [validating, setValidating] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [selectedPortalId, setSelectedPortalId] = React.useState<string | null>(
    null,
  );
  const [showPortalSelection, setShowPortalSelection] = React.useState(false);

  const { contractMeta, setContractMeta, resetContractMeta } =
    useCreateComponentBuildpackContext();
  const { validateOpenApiUrl } = useOpenApiValidation();
  const { devportals: portals, loading: portalsLoading } = useDevPortals();

  const autoFill = React.useCallback(
    (api: any) => {
      const title = api?.name?.trim() || api?.displayName?.trim() || "";
      const version = api?.version?.trim() || "1.0.0";
      const description = api?.description || "";
      const targetUrl = firstServerUrl(api);

      setContractMeta((prev: any) => ({
        ...prev,
        name: title || prev?.name || "Sample API",
        version,
        description,
        context: deriveContext(api),
        target: prev?.target || targetUrl || "",
      }));
    },
    [setContractMeta],
  );

  const handleFetchAndValidate = React.useCallback(async () => {
    if (!specUrl.trim()) return;

    try {
      setError(null);
      setValidating(true);
      setValidationResult(null);

      const result = await validateOpenApiUrl(specUrl.trim());
      setValidationResult(result);

      if (result.isAPIDefinitionValid) {
        autoFill(result.api);
        setShowPortalSelection(false);
      } else {
        const errorMsg =
          result.errors?.join(", ") || "Invalid OpenAPI definition";
        setError(errorMsg);
      }
    } catch (e: any) {
      setError(e?.message || "Failed to validate OpenAPI from URL");
      setValidationResult(null);
    } finally {
      setValidating(false);
    }
  }, [specUrl, autoFill, validateOpenApiUrl]);

  const previewOps = React.useMemo(() => {
    if (!validationResult?.isAPIDefinitionValid) return [];
    const api = validationResult.api as any;
    return mapOperations(api?.operations || [], { withFallbackName: true });
  }, [validationResult]);

  const handleContinueToPortalSelection = () => {
    if (validationResult?.isAPIDefinitionValid) {
      setShowPortalSelection(true);
      setError(null);
    }
  };

  const handlePortalSelect = (portalId: string) => {
    setSelectedPortalId(portalId);
    const selectedPortal = portals.find((p: Portal) => p.uuid === portalId);
    console.log("Selected Portal:", {
      id: portalId,
      name: selectedPortal?.name,
      url: selectedPortal?.uiUrl,
    });
    console.log("API URL:", specUrl);
    setActiveStep(1);
    setError(null);
  };

  const handleCreateAndPublish = async () => {
    const name = (contractMeta?.name || "").trim();
    const context = (contractMeta?.context || "").trim();
    const version = (contractMeta?.version || "").trim();
    const description = (contractMeta?.description || "").trim() || undefined;
    const target = (contractMeta?.target || "").trim();

    if (!name || !context || !version) {
      setError("Please complete all required fields.");
      return;
    }
    if (!selectedPortalId) {
      setError("Please select a developer portal.");
      return;
    }
    if (target) {
      try {
        if (/^https?:\/\//i.test(target)) new URL(target);
      } catch {
        setError("Target must be a valid URL (or leave it empty).");
        return;
      }
    }

    if (!validationResult?.isAPIDefinitionValid) {
      setError("Please validate the OpenAPI definition first.");
      return;
    }

    try {
      setCreating(true);
      setError(null);

      const serviceName = defaultServiceName(name);
      const backendServices = target
        ? [
          {
            name: serviceName,
            isDefault: true,
            retries: 2,
            endpoints: [{ url: target, description: "Primary backend" }],
          },
        ]
        : [];

      const validatedApi = validationResult.api as any;
      const operations = mapOperations(
        validatedApi?.operations || [],
        target ? { serviceName } : undefined,
      );

      // TODO: Implement actual createApi and publishToPortal logic
      // This is a placeholder for the API creation and portal publishing
      await new Promise((resolve) => setTimeout(resolve, 1000));

      console.log("Creating API with:", {
        name,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        description,
        contract: specUrl,
        backendServices,
        operations,
        portalId: selectedPortalId,
      });

      resetContractMeta();
      setActiveStep(0);
      setSpecUrl("");
      setValidationResult(null);
      setSelectedPortalId(null);
      onFinish?.();
    } catch (e: any) {
      setError(e?.message || "Failed to create and publish API");
    } finally {
      setCreating(false);
    }
  };

  return (
    <Box display="flex" flexDirection="column" alignItems="center">
      <Container
        maxWidth="lg"
        style={{
          display: "flex",
          flexDirection: "column",
          alignItems: "center",
        }}
      >
        <StepperBar
          steps={STEPS}
          activeStep={activeStep}
          onChange={(idx) => {
            if (idx <= activeStep) setActiveStep(idx);
          }}
        />

        <Box
          sx={{
            bgcolor: "background.paper",
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
          {activeStep === 0 && (
            <>
              {!showPortalSelection ? (
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
                              "https://petstore.swagger.io/v2/swagger.json",
                            )
                          }
                          disabled={validating}
                        >
                          Try with Sample URL
                        </Button>
                        <Box flex={1} />
                        <Button
                          variant="outlined"
                          onClick={handleFetchAndValidate}
                          disabled={!specUrl.trim() || validating}
                        >
                          {validating ? "Validating..." : "Fetch & Validate"}
                        </Button>
                      </Stack>

                      {error && (
                        <Alert severity="error" sx={{ mt: 2 }}>
                          {error}
                        </Alert>
                      )}
                    </Paper>

                    {validationResult?.isAPIDefinitionValid && (
                      <Stack
                        direction="row"
                        spacing={1}
                        sx={{ mt: 2 }}
                        justifyContent="flex-end"
                      >
                        <Button
                          variant="contained"
                          onClick={handleContinueToPortalSelection}
                        >
                          Continue
                        </Button>
                      </Stack>
                    )}
                  </Grid>

                  <Grid size={{ xs: 12, md: 6 }}>
                    {validating ? (
                      <Paper
                        variant="outlined"
                        sx={{ p: 3, borderRadius: 2, color: "text.secondary" }}
                      >
                        <Typography variant="body2">
                          Validating OpenAPI definition...
                        </Typography>
                      </Paper>
                    ) : validationResult?.isAPIDefinitionValid ? (
                      <ApiOperationsList
                        title="Fetched OAS Definition"
                        operations={previewOps}
                      />
                    ) : (
                      <Paper
                        variant="outlined"
                        sx={{ p: 3, borderRadius: 2, color: "text.secondary" }}
                      >
                        <Typography variant="body2">
                          Enter a direct URL to an OpenAPI/Swagger document
                          (YAML or JSON). We'll fetch and preview it here.
                        </Typography>
                      </Paper>
                    )}
                  </Grid>
                </Grid>
              ) : (
                <Box>
                  <Typography variant="h6" fontWeight={600} sx={{ mb: 2 }}>
                    Select Developer Portal
                  </Typography>

                  {portalsLoading ? (
                    <Box
                      sx={{ display: "flex", justifyContent: "center", p: 4 }}
                    >
                      <CircularProgress />
                    </Box>
                  ) : (
                    <Grid container spacing={2}>
                      {portals.map((portal: Portal) => (
                        <Grid key={portal.uuid} size={{ xs: 6, sm: 4, md: 3 }}>
                          <WizardPortalCard
                            title={portal.name}
                            description={portal.description}
                            portalUrl={portal.uiUrl}
                            selected={selectedPortalId === portal.uuid}
                            onSelect={() => {
                              if (selectedPortalId === portal.uuid) {
                                setSelectedPortalId(null);
                              } else {
                                setSelectedPortalId(portal.uuid);
                                setTimeout(() => {
                                  handlePortalSelect(portal.uuid);
                                }, 300);
                              }
                            }}
                            logoSrc={portal.logoSrc || BijiraDPLogo}
                            logoAlt={
                              portal.logoAlt ||
                              PORTAL_CONSTANTS.DEFAULT_LOGO_ALT
                            }
                          />
                        </Grid>
                      ))}
                    </Grid>
                  )}
                </Box>
              )}
            </>
          )}

          {activeStep === 1 && (
            <Box>
              <Typography variant="h6" fontWeight={600} sx={{ mb: 2 }}>
                API Details
              </Typography>

              <Grid container spacing={2}>
                <Grid size={{ xs: 12 }}>
                  <Paper variant="outlined" sx={{ p: 3, borderRadius: 2 }}>
                    <CreationMetaData scope="contract" title="Configure API" />

                    {error && (
                      <Alert severity="error" sx={{ mt: 2 }}>
                        {error}
                      </Alert>
                    )}
                  </Paper>

                  <Stack
                    direction="row"
                    spacing={1}
                    sx={{ mt: 2 }}
                    justifyContent="space-between"
                  >
                    <Button variant="outlined" onClick={() => setActiveStep(0)}>
                      Back
                    </Button>
                    <Button
                      variant="contained"
                      disabled={
                        creating ||
                        !selectedPortalId ||
                        !(contractMeta?.name || "").trim() ||
                        !(contractMeta?.context || "").trim() ||
                        !(contractMeta?.version || "").trim()
                      }
                      onClick={handleCreateAndPublish}
                    >
                      {creating ? "Publishing..." : "Create & Publish"}
                    </Button>
                  </Stack>
                </Grid>
              </Grid>
            </Box>
          )}
        </Box>
      </Container>
    </Box>
  );
}
function StepperBar({
  steps,
  activeStep,
  onChange,
}: {
  steps: { title: string; subtitle: string }[];
  activeStep: number;
  onChange: (idx: number) => void;
}) {
  const overlap = 36;

  return (
    <Box
      sx={{
        display: "flex",
        alignItems: "stretch",
        mb: 0,
        width: "100%",
        maxWidth: 1010,
      }}
    >
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
          totalSteps={steps.length}
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
  overlap,
  zIndex,
  title,
  subtitle,
}: {
  index: number;
  last: boolean;
  active: boolean;
  completed: boolean;
  onClick: () => void;
  overlap: number;
  zIndex: number;
  title: string;
  subtitle: string;
  totalSteps: number;
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
        flex: 1,
        display: "flex",
      }}
    >
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
          flex: 1,
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

export default function PublishPortalFlow({
  onFinish,
}: {
  onFinish?: () => void;
}) {
  return (
    <CreateComponentBuildpackProvider>
      <PublishPortalFlowContent onFinish={onFinish} />
    </CreateComponentBuildpackProvider>
  );
}
