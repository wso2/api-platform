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
  List,
  ListItemButton,
  ListItemText,
  Divider,
  Chip,
  TextField,
  MenuItem,
} from "@mui/material";
import CheckCircleRoundedIcon from "@mui/icons-material/CheckCircleRounded";
import { TextInput } from "../../components/src/components/TextInput";
import { Button } from "../../components/src/components/Button";
import { ApiOperationsList } from "../../components/src/components/Common/ApiOperationsList";
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
import { useApisContext } from "../../context/ApiContext";
import type { ApiSummary } from "../../hooks/apis";
import { useProjects } from "../../context/ProjectContext";

type Step = { title: string; subtitle: string };
const STEPS: Step[] = [
  {
    title: "Select or Create API",
    subtitle: "Choose existing API or create new one",
  },
  { title: "Select Portal", subtitle: "Choose developer portal to publish" },
];

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
  const [portalVisibility, setPortalVisibility] = React.useState<string>("PUBLIC");
  const [portalEndpoint, setPortalEndpoint] = React.useState<string>("");
  const [selectedExistingApi, setSelectedExistingApi] = React.useState<ApiSummary | null>(null);
  const [selectionMode, setSelectionMode] = React.useState<"url" | "existing">("url");

  const { contractMeta, setContractMeta, resetContractMeta } =
    useCreateComponentBuildpackContext();
  const { validateOpenApiUrl } = useOpenApiValidation();
  const { devportals: portals, loading: portalsLoading } = useDevPortals();
  const { apis, loading: apisLoading, importOpenApi, refreshApis } = useApisContext();
  const { selectedProject } = useProjects();
  const typedApis = React.useMemo<ApiSummary[]>(() => apis, [apis]);

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

  const handleCreateApi = async () => {
    const name = (contractMeta?.name || "").trim();
    const version = (contractMeta?.version || "").trim();
    const description = (contractMeta?.description || "").trim() || undefined;

    if (!name || !version) {
      setError("Please provide API name and version.");
      return;
    }
    
    if (!portalVisibility || !portalEndpoint.trim()) {
      setError("Please provide visibility and endpoint for developer portal.");
      return;
    }

    if (!validationResult?.isAPIDefinitionValid) {
      setError("Please validate the OpenAPI definition first.");
      return;
    }

    try {
      setCreating(true);
      setError(null);

      if (!selectedProject?.id) {
        setError("No project found.");
        return;
      }

      const validatedApi = validationResult.api as any;
      
      const apiPayload = {
        name,
        version,
        context: validatedApi?.context || deriveContext(validatedApi),
        projectId: selectedProject.id,
        description,
        operations: validatedApi?.operations || [],
        "backend-services": validatedApi?.["backend-services"] || [],
      };

      await importOpenApi(
        {
          api: apiPayload,
          url: specUrl.trim(),
        }
      );

      await refreshApis();
      
      setActiveStep(1);
      setError(null);
    } catch (e: any) {
      const errorMessage = e?.response?.data?.message 
        || e?.response?.data?.description
        || e?.message 
        || "Failed to create API";
      setError(errorMessage);
    } finally {
      setCreating(false);
    }
  };

  const handleContinueWithExistingApi = () => {
    if (!selectedExistingApi) {
      setError("Please select an API.");
      return;
    }
    setError(null);
    setActiveStep(1);
  };

  const handleSelectExistingApi = (api: ApiSummary) => {
    setSelectedExistingApi(api);
    setError(null);
  };

  const isStep0Complete = 
    selectionMode === "url" 
      ? validationResult?.isAPIDefinitionValid && 
        (contractMeta?.name || "").trim() && 
        (contractMeta?.version || "").trim() &&
        portalVisibility.trim() &&
        portalEndpoint.trim()
      : selectedExistingApi !== null &&
        portalVisibility.trim() &&
        portalEndpoint.trim();

  const step0ButtonLabel = selectionMode === "url" ? "Create API" : "Continue";
  const step0ButtonAction = selectionMode === "url" ? handleCreateApi : handleContinueWithExistingApi;

  const handlePublishToPortal = async () => {
    if (!selectedPortalId) {
      setError("Please select a developer portal.");
      return;
    }

    try {
      setCreating(true);
      setError(null);
      const selectedPortal = portals.find((p: Portal) => p.uuid === selectedPortalId);
      await new Promise((resolve) => setTimeout(resolve, 1000));
      console.log("Publishing to Portal:", {
        portalId: selectedPortalId,
        portalName: selectedPortal?.name,
        apiName: contractMeta?.name,
        portalVisibility,
        portalEndpoint,
      });

      resetContractMeta();
      setActiveStep(0);
      setSpecUrl("");
      setValidationResult(null);
      setSelectedPortalId(null);
      onFinish?.();
    } catch (e: any) {
      setError(e?.message || "Failed to publish to portal");
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
              {!validationResult?.isAPIDefinitionValid && !selectedExistingApi && (
                <Box sx={{ display: 'flex', gap: 2, alignItems: 'stretch' }}>
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Stack spacing={2}>
                      <Paper 
                        variant="outlined" 
                        sx={{ 
                          p: 3, 
                          borderRadius: 2,
                          borderColor: selectionMode === "url" ? "primary.main" : "divider",
                          borderWidth: selectionMode === "url" ? 2 : 1,
                          cursor: "pointer",
                          transition: "all 0.2s",
                          "&:hover": {
                            borderColor: "primary.main",
                          }
                        }}
                        onClick={() => {
                          setSelectionMode("url");
                          setSelectedExistingApi(null);
                          setError(null);
                        }}
                      >
                        <Typography variant="subtitle2" sx={{ mb: 1 }} fontWeight={600}>
                          🌐 Create New API from OpenAPI Specification URL
                        </Typography>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                          Import from a public OpenAPI specification URL
                        </Typography>
                        
                        {selectionMode === "url" && (
                          <>
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
                                onClick={(e: React.MouseEvent) => {
                                  e.stopPropagation();
                                  setSpecUrl(
                                    "https://petstore.swagger.io/v2/swagger.json",
                                  );
                                }}
                                disabled={validating}
                              >
                                Try Sample
                              </Button>
                              <Box flex={1} />
                              <Button
                                variant="outlined"
                                onClick={(e: React.MouseEvent) => {
                                  e.stopPropagation();
                                  handleFetchAndValidate();
                                }}
                                disabled={!specUrl.trim() || validating}
                              >
                                {validating ? "Validating..." : "Fetch & Validate"}
                              </Button>
                            </Stack>
                          </>
                        )}
                      </Paper>

                      {selectionMode === "url" && validating && (
                        <Paper
                          variant="outlined"
                          sx={{
                            p: 3,
                            borderRadius: 2,
                            color: "text.secondary",
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center'
                          }}
                        >
                          <CircularProgress size={24} sx={{ mr: 2 }} />
                          <Typography variant="body2">
                            Validating OpenAPI definition...
                          </Typography>
                        </Paper>
                      )}

                      {error && selectionMode === "url" && (
                        <Alert severity="error">
                          {error}
                        </Alert>
                      )}
                    </Stack>
                  </Box>

                  <Box
                    sx={{
                      display: { xs: 'none', md: 'flex' },
                      alignItems: 'stretch',
                      justifyContent: 'center',
                      position: 'relative',
                      width: 40,
                      py: 0,
                    }}
                  >
                    <Divider 
                      orientation="vertical" 
                      sx={{ 
                        height: '100%',
                        alignSelf: 'stretch'
                      }} 
                    />
                    <Box
                      sx={{
                        position: 'absolute',
                        top: '50%',
                        transform: 'translateY(-50%)',
                        bgcolor: 'background.paper',
                        px: 1,
                        color: 'text.secondary',
                        fontWeight: 500,
                        fontSize: '0.875rem',
                      }}
                    >
                      OR
                    </Box>
                  </Box>

                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Stack spacing={2}>
                      <Paper 
                        variant="outlined" 
                        sx={{ 
                          p: 3, 
                          borderRadius: 2,
                          borderColor: selectionMode === "existing" ? "primary.main" : "divider",
                          borderWidth: selectionMode === "existing" ? 2 : 1,
                          cursor: "pointer",
                          transition: "all 0.2s",
                          "&:hover": {
                            borderColor: "primary.main",
                          }
                        }}
                        onClick={() => {
                          setSelectionMode("existing");
                          setValidationResult(null);
                          setSpecUrl("");
                          setError(null);
                        }}
                      >
                        <Typography variant="subtitle2" sx={{ mb: 1 }} fontWeight={600}>
                          📋 Select Existing API
                        </Typography>
                        <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                          Choose from your existing APIs and configure portal settings
                        </Typography>

                        {selectionMode === "existing" && (() => {
                          const currentSelection: ApiSummary | null = selectedExistingApi;
                          
                          return (
                          <>
                            {apisLoading ? (
                              <Box sx={{ display: "flex", justifyContent: "center", p: 2 }}>
                                <CircularProgress size={24} />
                              </Box>
                            ) : typedApis.length === 0 ? (
                              <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center", py: 2 }}>
                                No APIs found. Create one first.
                              </Typography>
                            ) : (
                              <Paper 
                                variant="outlined" 
                                sx={{ 
                                  maxHeight: 300, 
                                  overflow: "auto",
                                  borderRadius: 1
                                }}
                              >
                                <List dense disablePadding>
                                  {typedApis.map((api, index) => {
                                    const apiId = api.id;
                                    const apiName = api.name;
                                    const apiVersion = api.version;
                                    const apiContext = api.context;
                                    const isSelected = currentSelection !== null && (currentSelection as ApiSummary).name === apiName;
                                    
                                    return (
                                    <React.Fragment key={apiId}>
                                      {index > 0 && <Divider />}
                                      <ListItemButton
                                        selected={isSelected}
                                        onClick={(e: React.MouseEvent) => {
                                          e.stopPropagation();
                                          handleSelectExistingApi(api);
                                        }}
                                      >
                                        <ListItemText
                                          primary={
                                            <Box display="flex" alignItems="center" gap={1}>
                                              <Typography variant="body2" fontWeight={500}>
                                                {apiName}
                                              </Typography>
                                              <Chip 
                                                label={apiVersion} 
                                                size="small" 
                                                sx={{ height: 20, fontSize: "0.7rem" }}
                                              />
                                            </Box>
                                          }
                                          secondary={
                                            <Typography variant="caption" color="text.secondary" noWrap>
                                              {apiContext}
                                            </Typography>
                                          }
                                        />
                                      </ListItemButton>
                                    </React.Fragment>
                                  )})}
                                </List>
                              </Paper>
                            )}
                          </>
                        )})()}
                      </Paper>

                      {error && selectionMode === "existing" && (
                        <Alert severity="error">
                          {error}
                        </Alert>
                      )}
                    </Stack>
                  </Box>
                </Box>
              )}

              {selectionMode === "url" && validationResult?.isAPIDefinitionValid && (
                <Stack spacing={3}>
                  <Grid container spacing={3}>
                    <Grid size={{ xs: 12, md: 6 }}>
                      <Box>
                        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 2 }}>
                          Fetched OAS Definition
                        </Typography>
                        <ApiOperationsList
                          title=""
                          operations={previewOps}
                        />
                      </Box>
                    </Grid>

                    <Grid size={{ xs: 12, md: 6 }}>
                      <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, height: '100%' }}>
                        <Typography
                          variant="subtitle1"
                          fontWeight={600}
                          sx={{ mb: 3 }}
                        >
                          Configure API
                        </Typography>

                        <Stack spacing={2.5}>
                          <Grid container spacing={2}>
                            <Grid size={{ xs: 12, sm: 6 }}>
                              <TextField
                                label="Name"
                                value={contractMeta?.name || ""}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                                  setContractMeta((prev: any) => ({ ...prev, name: e.target.value }))
                                }
                                fullWidth
                                required
                                variant="outlined"
                                placeholder="My API"
                              />
                            </Grid>
                            <Grid size={{ xs: 12, sm: 6 }}>
                              <TextField
                                label="Version"
                                value={contractMeta?.version || ""}
                                onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                                  setContractMeta((prev: any) => ({ ...prev, version: e.target.value }))
                                }
                                fullWidth
                                required
                                variant="outlined"
                                placeholder="1.0.0"
                              />
                            </Grid>
                          </Grid>

                          <TextField
                            label="Description"
                            value={contractMeta?.description || ""}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                              setContractMeta((prev: any) => ({ ...prev, description: e.target.value }))
                            }
                            fullWidth
                            multiline
                            rows={3}
                            variant="outlined"
                            placeholder="Describe your API"
                          />

                          <Divider />

                          <Typography
                            variant="subtitle1"
                            fontWeight={600}
                          >
                            Developer Portal Settings
                          </Typography>

                          <TextField
                            select
                            label="Access Visibility"
                            value={portalVisibility}
                            onChange={(e) => setPortalVisibility(e.target.value)}
                            fullWidth
                            required
                            variant="outlined"
                            helperText="Control who can discover your API"
                          >
                            <MenuItem value="PUBLIC">
                              <Box display="flex" alignItems="center" gap={1}>
                                <span>🌍</span>
                                <Typography variant="body2">Public</Typography>
                              </Box>
                            </MenuItem>
                            <MenuItem value="PRIVATE">
                              <Box display="flex" alignItems="center" gap={1}>
                                <span>🔒</span>
                                <Typography variant="body2">Private</Typography>
                              </Box>
                            </MenuItem>
                          </TextField>

                          <TextField
                            label="Endpoint"
                            value={portalEndpoint}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPortalEndpoint(e.target.value)}
                            fullWidth
                            required
                            variant="outlined"
                            placeholder="https://api.example.com"
                            helperText="Endpoint URL displayed to developers"
                            error={portalEndpoint.trim() !== '' && !/^https?:\/\/.+/.test(portalEndpoint.trim())}
                          />
                        </Stack>
                      </Paper>
                    </Grid>
                  </Grid>

                  {error && (
                    <Alert severity="error">
                      {error}
                    </Alert>
                  )}

                  <Stack direction="row" spacing={2} justifyContent="flex-end">
                    <Button
                      variant="outlined"
                      onClick={() => {
                        setValidationResult(null);
                        setSpecUrl("");
                        setError(null);
                      }}
                    >
                      Cancel
                    </Button>
                    <Button
                      variant="contained"
                      disabled={creating || !isStep0Complete}
                      onClick={step0ButtonAction}
                    >
                      {creating ? "Creating..." : step0ButtonLabel}
                    </Button>
                  </Stack>
                </Stack>
              )}

              {/* Existing API Mode: After selection - Full width layout */}
              {selectionMode === "existing" && selectedExistingApi && (
                <Stack spacing={3}>
                  <Grid container spacing={3}>
                    {/* Left: API Info */}
                    <Grid size={{ xs: 12, md: 6 }}>
                      <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, height: '100%' }}>
                        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 3 }}>
                          Selected API
                        </Typography>
                        
                        <Stack spacing={2.5}>
                          <Box>
                            <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1 }}>
                              API Name
                            </Typography>
                            <Typography variant="h6" fontWeight={500} sx={{ mt: 0.5 }}>
                              {selectedExistingApi.name}
                            </Typography>
                          </Box>

                          <Divider />

                          <Box>
                            <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1 }}>
                              Version
                            </Typography>
                            <Typography variant="body1" sx={{ mt: 0.5 }}>
                              {selectedExistingApi.version}
                            </Typography>
                          </Box>

                          <Box>
                            <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1 }}>
                              Context Path
                            </Typography>
                            <Typography variant="body1" sx={{ mt: 0.5, fontFamily: 'monospace' }}>
                              {selectedExistingApi.context}
                            </Typography>
                          </Box>

                          {selectedExistingApi.description && (
                            <>
                              <Divider />
                              <Box>
                                <Typography variant="overline" color="text.secondary" sx={{ letterSpacing: 1 }}>
                                  Description
                                </Typography>
                                <Typography variant="body2" sx={{ mt: 0.5, color: 'text.secondary' }}>
                                  {selectedExistingApi.description}
                                </Typography>
                              </Box>
                            </>
                          )}
                        </Stack>
                      </Paper>
                    </Grid>

                    <Grid size={{ xs: 12, md: 6 }}>
                      <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, height: '100%' }}>
                        <Typography variant="subtitle1" fontWeight={600} sx={{ mb: 3 }}>
                          Developer Portal Settings
                        </Typography>

                        <Stack spacing={2.5}>
                          <TextField
                            select
                            label="Access Visibility"
                            value={portalVisibility}
                            onChange={(e) => setPortalVisibility(e.target.value)}
                            fullWidth
                            required
                            variant="outlined"
                            helperText="Control who can discover your API in the portal"
                          >
                            <MenuItem value="PUBLIC">
                              <Box display="flex" alignItems="center" gap={1}>
                                <span>🌍</span>
                                <Typography variant="body2">Public</Typography>
                              </Box>
                            </MenuItem>
                            <MenuItem value="PRIVATE">
                              <Box display="flex" alignItems="center" gap={1}>
                                <span>🔒</span>
                                <Typography variant="body2">Private</Typography>
                              </Box>
                            </MenuItem>
                          </TextField>

                          <TextField
                            label="Endpoint"
                            value={portalEndpoint}
                            onChange={(e: React.ChangeEvent<HTMLInputElement>) => setPortalEndpoint(e.target.value)}
                            fullWidth
                            required
                            variant="outlined"
                            placeholder="https://api.example.com"
                            helperText="The endpoint URL that will be displayed to developers"
                            error={portalEndpoint.trim() !== '' && !/^https?:\/\/.+/.test(portalEndpoint.trim())}
                          />

                          <Box 
                            sx={{ 
                              p: 2, 
                              borderRadius: 1,
                              border: '1px solid',
                              borderColor: '#e0ebd5'
                            }}
                          >
                            <Typography variant="caption" color="text.secondary">
                              These settings are only for the developer portal and do not affect the API itself
                            </Typography>
                          </Box>
                        </Stack>
                      </Paper>
                    </Grid>
                  </Grid>

                  {error && (
                    <Alert severity="error">
                      {error}
                    </Alert>
                  )}

                  <Stack direction="row" spacing={2} justifyContent="flex-end">
                    <Button
                      variant="outlined"
                      onClick={() => {
                        setSelectedExistingApi(null);
                        setError(null);
                      }}
                    >
                      Cancel
                    </Button>
                    <Button
                      variant="contained"
                      disabled={creating || !isStep0Complete}
                      onClick={step0ButtonAction}
                    >
                      {creating ? "Loading..." : step0ButtonLabel}
                    </Button>
                  </Stack>
                </Stack>
              )}
            </>
          )}

          {activeStep === 1 && (
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

              <Stack
                direction="row"
                spacing={1}
                sx={{ mt: 3 }}
                justifyContent="space-between"
              >
                <Button variant="outlined" onClick={() => setActiveStep(0)}>
                  Back
                </Button>
                <Button
                  variant="contained"
                  disabled={creating || !selectedPortalId}
                  onClick={handlePublishToPortal}
                >
                  {creating ? "Publishing..." : "Publish to Portal"}
                </Button>
              </Stack>
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
