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
  InputAdornment,
  
} from "@mui/material";
import CheckCircleRoundedIcon from "@mui/icons-material/CheckCircleRounded";
import SearchIcon from '@mui/icons-material/Search';
import { TextInput } from "../../components/src/components/TextInput";
import { Button } from "../../components/src/components/Button";
import { ApiOperationsList } from "../../components/src/components/Common/ApiOperationsList";
import WizardPortalCard from "../portals/WizardPortalCard";
import { buildPublishPayload } from "../apis/ApiPublish/mapper";
import ApiPublishForm from "../apis/ApiPublish/ApiPublishForm";
import {
  useCreateComponentBuildpackContext,
  CreateComponentBuildpackProvider,
} from "../../context/CreateComponentBuildpackContext";
import { formatVersionToMajorMinor, isValidMajorMinorVersion, firstServerUrl, deriveContext, mapOperations } from "../../helpers/openApiHelpers";
import VersionInput from "../../common/VersionInput";
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
import { ApiPublishProvider, useApiPublishing } from "../../context/ApiPublishContext";
import { DevPortalProvider } from "../../context/DevPortalContext";
import { useNavigate } from "react-router-dom";
import { useNotifications } from "../../context/NotificationContext";
import { useOrganization } from "../../context/OrganizationContext";
import { projectSlugFromName } from "../../utils/projectSlug";

type Step = { title: string; subtitle: string };
const STEPS: Step[] = [
  {
    title: "Select or Create API",
    subtitle: "Choose existing API or create new one",
  },
  { title: "Select Portal", subtitle: "Choose developer portal to publish" },
];

function isValidHttpUrl(urlString: string): boolean {
  try {
    const url = new URL(urlString.trim());
    return ['http:', 'https:'].includes(url.protocol);
  } catch {
    return false;
  }
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
  const [existingQuery, setExistingQuery] = React.useState<string>("");

  const { contractMeta, setContractMeta, resetContractMeta } =
    useCreateComponentBuildpackContext();
  const { validateOpenApiUrl } = useOpenApiValidation();
  const { devportals: allPortals, loading: portalsLoading } = useDevPortals();
  const { apis, loading: apisLoading, importOpenApi, refreshApis, fetchGatewaysForApi } = useApisContext();
  const { selectedProject } = useProjects();
  const { publishApiToDevPortal, refreshPublishedApis } = useApiPublishing();
  const navigate = useNavigate();
  const { showNotification } = useNotifications();
  const { organization } = useOrganization();
  const theme = useTheme();
  const typedApis = React.useMemo<ApiSummary[]>(() => apis, [apis]);
  const filteredTypedApis = React.useMemo(() => {
    const q = (existingQuery || "").trim().toLowerCase();
    if (!q) return typedApis;
    return typedApis.filter((a) => {
      return (
        (a.name || "").toLowerCase().includes(q) ||
        (a.context || "").toLowerCase().includes(q)
      );
    });
  }, [typedApis, existingQuery]);
  
  const portals = React.useMemo(() => allPortals.filter(p => p.isEnabled), [allPortals]);

  const [showAdvanced, setShowAdvanced] = React.useState(false);
  const [gateways, setGateways] = React.useState<any[]>([]);
  const [loadingGateways, setLoadingGateways] = React.useState(false);
  const [formData, setFormData] = React.useState<any>({
    apiName: contractMeta?.name || '',
    productionURL: portalEndpoint || '',
    sandboxURL: portalEndpoint || '',
    apiDescription: contractMeta?.description || '',
    visibility: portalVisibility || 'PUBLIC',
    technicalOwner: '',
    technicalOwnerEmail: '',
    businessOwner: '',
    businessOwnerEmail: '',
    labels: ['default'],
    subscriptionPolicies: [],
    tags: [],
    selectedDocumentIds: [],
  });
  const [allPublishedToActivePortals, setAllPublishedToActivePortals] = React.useState(false);
  const [publishedStatusLoading, setPublishedStatusLoading] = React.useState(false);
  const [newTag, setNewTag] = React.useState('');

  const handleUrlChange = (type: 'production' | 'sandbox', url: string) => {
    setFormData((prev: any) => ({
      ...prev,
      [`${type}URL`]: url,
      ...(type === 'production' && !prev.sandboxURL ? { sandboxURL: url } : {}),
    }));
  };

  const handleCheckboxChange = (field: 'labels' | 'subscriptionPolicies' | 'selectedDocumentIds', value: string, checked: boolean) => {
    setFormData((prev: any) => ({
      ...prev,
      [field]: checked ? [...(prev[field] || []), value] : (prev[field] || []).filter((item: string) => item !== value),
    }));
  };

  const handleAddTag = () => {
    if (newTag.trim() && !formData.tags?.includes(newTag.trim())) {
      setFormData((prev: any) => ({ ...prev, tags: [...(prev.tags || []), newTag.trim()] }));
      setNewTag('');
    }
  };

  const handleRemoveTag = (tagToRemove: string) => {
    setFormData((prev: any) => ({ ...prev, tags: (prev.tags || []).filter((t: string) => t !== tagToRemove) }));
  };

  React.useEffect(() => {
    if (activeStep === 1) {
      let isMounted = true;
      const endpointFromSelected = selectionMode === 'existing' ? firstServerUrl(selectedExistingApi) : '';
      const endpointFromValidation = (validationResult && validationResult.isAPIDefinitionValid) ? firstServerUrl((validationResult as any).api) : '';

      setFormData((prev: any) => ({
        ...prev,
        apiName: selectionMode === 'existing' ? selectedExistingApi?.name || prev.apiName : contractMeta?.name || prev.apiName,
        apiDescription: selectionMode === 'existing' ? selectedExistingApi?.description || prev.apiDescription : contractMeta?.description || prev.apiDescription,
        productionURL: endpointFromSelected || endpointFromValidation || contractMeta?.target || portalEndpoint || prev.productionURL,
        sandboxURL: endpointFromSelected || endpointFromValidation || contractMeta?.target || portalEndpoint || prev.sandboxURL,
        visibility: portalVisibility || prev.visibility,
      }));

      const apiId = selectionMode === 'existing' ? selectedExistingApi?.id : undefined;
      if (apiId) {
        (async () => {
          setLoadingGateways(true);
          try {
            const apiGateways = await fetchGatewaysForApi(apiId);
            if (isMounted) setGateways(apiGateways || []);
          } catch (err) {
            if (isMounted) setGateways([]);
          } finally {
            if (isMounted) setLoadingGateways(false);
          }
        })();
      }

      (async () => {
        setPublishedStatusLoading(true);
        try {
          const targetApiId = selectionMode === 'existing' ? selectedExistingApi?.id : (apis.find((a: any) => a.name === contractMeta?.name && a.version === contractMeta?.version)?.id);
          if (!targetApiId) {
            if (isMounted) setAllPublishedToActivePortals(false);
            return;
          }

          const pubs = await refreshPublishedApis(targetApiId);
          const publishedSet = new Set((pubs || []).map((p: any) => p.uuid));
          const allPublished = portals.length > 0 && portals.every((p: any) => publishedSet.has(p.uuid));
          if (isMounted) setAllPublishedToActivePortals(!!allPublished);
        } catch (e) {
          if (isMounted) setAllPublishedToActivePortals(false);
        } finally {
          if (isMounted) setPublishedStatusLoading(false);
        }
      })();

      return () => { isMounted = false; };
    }
  }, [activeStep, selectionMode, selectedExistingApi, contractMeta, portalEndpoint, portalVisibility, fetchGatewaysForApi, validationResult, apis, portals, refreshPublishedApis]);

  React.useEffect(() => {
    if (activeStep === 1 && portals.length > 0) {
      if (portals.length === 1) {
        setSelectedPortalId(portals[0].uuid);
      }
    }
  }, [activeStep, portals]);

  const portalsPath = React.useMemo(() => {
    if (!organization) return "/portals";
    const projectSegment = selectedProject
      ? `/${projectSlugFromName(selectedProject.name, selectedProject.id)}`
      : "";
    return `/${organization.handle}${projectSegment}/portals`;
  }, [organization, selectedProject]);

  const handleGoToPortals = React.useCallback(() => {
    if (onFinish) {
      onFinish();
    }
    navigate(portalsPath);
  }, [onFinish, navigate, portalsPath]);

  const autoFill = React.useCallback(
    (api: any) => {
      const title = api?.name?.trim() || api?.displayName?.trim() || "";
      const version = formatVersionToMajorMinor(api?.version);
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
    const context = (contractMeta?.context || "").trim();

    if (!name || !version || !context) {
      setError("Please provide API name, version and context.");
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
        context: contractMeta?.context || validatedApi?.context || deriveContext(validatedApi),
        projectId: selectedProject.id,
        description,
        operations: validatedApi?.operations || [],
        "backend-services": validatedApi?.["backend-services"] || [],
      };

      const createdApi = await importOpenApi(
        {
          api: apiPayload,
          url: specUrl.trim(),
        }
      );

      await refreshApis();
      if (createdApi) {
        autoFill(createdApi);
      }
      showNotification(`API "${name}" created successfully!`, 'success');
      
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
    setActiveStep(1);
  };

  const isStep0Complete =
    selectionMode === "url"
      ? validationResult?.isAPIDefinitionValid &&
        (contractMeta?.name || "").trim() &&
        isValidMajorMinorVersion((contractMeta?.version || "").trim()) &&
        (contractMeta?.context || "").trim()
      : selectedExistingApi !== null;

  const step0ButtonLabel = selectionMode === "url" ? "Create API" : "Continue";
  const step0ButtonAction = selectionMode === "url" ? handleCreateApi : handleContinueWithExistingApi;



  const handlePublishFromModal = async (portalId: string, payload: any) => {
    try {
      setCreating(true);
      setError(null);

      let apiId: string | undefined;
      if (selectionMode === "existing" && selectedExistingApi) {
        apiId = selectedExistingApi.id;
      } else {
        const createdApi = apis.find(
          (a) => a.name === contractMeta?.name && a.version === contractMeta?.version,
        );
        if (!createdApi) {
          setError("Could not find the created API. Please try again.");
          return;
        }
        apiId = createdApi.id;
      }

      await publishApiToDevPortal(apiId, payload);

      const selectedPortal = portals.find((p: Portal) => p.uuid === portalId);
      const portalName = selectedPortal?.name || 'portal';
      showNotification(`API "${contractMeta?.name || selectedExistingApi?.name}" successfully published to ${portalName}!`, 'success');

      resetContractMeta();
      setActiveStep(0);
      setSpecUrl("");
      setValidationResult(null);
      setSelectedPortalId(null);
      setSelectedExistingApi(null);
      setPortalVisibility("PUBLIC");
      setPortalEndpoint("");
      onFinish?.();
    } catch (e: any) {
      const errorMessage = e?.response?.data?.message 
        || e?.response?.data?.description
        || e?.message 
        || "Failed to add to portal";
      setError(errorMessage);
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
              {!validationResult?.isAPIDefinitionValid && (
                <>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    Choose whether to create a new API from an OpenAPI specification URL or select an existing API to configure portal settings.
                  </Typography>
                  <Box sx={{ display: 'flex', gap: 2, mt: 0, alignItems: 'stretch' }}>
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
                              testId="publish-spec-url"
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
                              <Box sx={{ width: '100%' }}>
                                <Box sx={{ mb: 1 }}>
                                  <Box>
                                    <Box sx={{ borderRadius: 1, backgroundColor: theme.palette.success.light }}>
                                      <TextInput
                                        size="small"
                                        placeholder="Search APIs"
                                        value={existingQuery}
                                        onChange={(v: string) => setExistingQuery(v)}
                                        fullWidth
                                        testId="publish-existing-search"
                                        InputProps={{
                                          startAdornment: (
                                            <InputAdornment position="start">
                                              <SearchIcon fontSize="small" />
                                            </InputAdornment>
                                          ),
                                        }}
                                      />
                                    </Box>
                                  </Box>
                                </Box>
                                <Paper
                                  variant="outlined"
                                  sx={{
                                    maxHeight: 200, 
                                    overflow: "auto",
                                    borderRadius: 1,
                                    width: '100%'
                                  }}
                                >
                                  {filteredTypedApis.length === 0 ? (
                                    <Box sx={{ p: 2 }}>
                                      <Typography variant="body2" color="text.secondary" sx={{ textAlign: "center" }}>
                                        No APIs match "{existingQuery}"
                                      </Typography>
                                    </Box>
                                  ) : (
                                    <List dense disablePadding>
                                      {filteredTypedApis.map((api, index) => {
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
                                  )}
                                </Paper>
                              </Box>
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
                </>
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
                            <Grid size={{ xs: 12 }}>
                              <TextInput
                                label="Name"
                                value={contractMeta?.name || ""}
                                onChange={(v: string) => setContractMeta((prev: any) => ({ ...prev, name: v }))}
                                fullWidth
                                testId="publish-config-name"
                                size="medium"
                                placeholder="My API"
                              />
                            </Grid>
                            <Grid size={{ xs: 12 }}>
                              <VersionInput
                                value={contractMeta?.version}
                                onChange={(v: string) => setContractMeta((prev: any) => ({ ...prev, version: v }))}
                                disabled={false}
                                label="Version"
                              />
                            </Grid>
                          <Grid size={{ xs: 12 }}>
                            <TextInput
                              label="Context"
                              value={contractMeta?.context || ""}
                              onChange={(v: string) => setContractMeta((prev: any) => ({ ...prev, context: v }))}
                              onBlur={() => {
                                const ctx = (contractMeta?.context || "").trim();
                                if (ctx && !ctx.startsWith("/")) {
                                  setContractMeta((prev: any) => ({ ...prev, context: `/${ctx}` }));
                                }
                              }}
                              fullWidth
                              testId="publish-config-context"
                              size="medium"
                              placeholder="/my-api"
                            />
                          </Grid>
                          <Grid size={{ xs: 12 }}>
                            <TextInput
                              label="Target (Backend URL)"
                              value={contractMeta?.target || ""}
                              onChange={(v: string) => setContractMeta((prev: any) => ({ ...prev, target: v }))}
                              fullWidth
                              testId="publish-config-target"
                              size="medium"
                              placeholder="https://api.example.com"
                            />
                          </Grid>
                          </Grid>

                          <TextInput
                            label="Description"
                            value={contractMeta?.description || ""}
                            onChange={(v: string) => setContractMeta((prev: any) => ({ ...prev, description: v }))}
                            fullWidth
                            multiline
                            rows={3}
                            testId="publish-config-description"
                            size="medium"
                            placeholder="Describe your API"
                            className="description-input"
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

            </>
          )}

          {activeStep === 1 && (
            <Box>
              {!publishedStatusLoading && !allPublishedToActivePortals && (
                <>
                  {portals.length !== 1 && (
                    <Typography variant="h6" fontWeight={600} sx={{ mb: 2 }}>
                      Select Developer Portal
                    </Typography>
                  )}

                  {portalsLoading ? (
                    <Box sx={{ display: "flex", justifyContent: "center", p: 4 }}>
                      <CircularProgress />
                    </Box>
                  ) : portals.length === 0 ? (
                    <Paper
                      variant="outlined"
                      sx={{
                        p: 4,
                        textAlign: "center",
                        borderRadius: 2,
                        bgcolor: "background.default",
                      }}
                    >
                      <Typography variant="h6" sx={{ mb: 2 }} color="text.secondary">
                        No Active Developer Portals
                      </Typography>
                      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
                        You need to activate a developer portal before you can publish APIs.
                        Go to the Portals page to activate one.
                      </Typography>
                      <Button variant="contained" onClick={handleGoToPortals}>
                        Go to Portals
                      </Button>
                    </Paper>
                  ) : portals.length === 1 ? (
                    <Paper
                      variant="outlined"
                      sx={{ p: 2, borderRadius: 2, display: 'flex', alignItems: 'center', gap: 2 }}
                    >
                      <Box sx={{ width: 56, height: 56, flexShrink: 0 }}>
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={portals[0].logoSrc || BijiraDPLogo}
                          alt={portals[0].logoAlt || PORTAL_CONSTANTS.DEFAULT_LOGO_ALT}
                          style={{ width: 56, height: 56, objectFit: 'contain', borderRadius: 6 }}
                        />
                      </Box>
                      <Box>
                        <Typography variant="subtitle1" fontWeight={600}>
                          {portals[0].name}
                        </Typography>
                        {portals[0].description && (
                          <Typography variant="body2" color="text.secondary">
                            {portals[0].description}
                          </Typography>
                        )}
                      </Box>
                    </Paper>
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
                            logoAlt={portal.logoAlt || PORTAL_CONSTANTS.DEFAULT_LOGO_ALT}
                          />
                        </Grid>
                      ))}
                    </Grid>
                  )}
                </>
              )}

              {publishedStatusLoading ? (
                <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, mb: 2, mt: 3 }}>
                  <Box display="flex" alignItems="center" justifyContent="center" sx={{ py: 1 }}>
                    <CircularProgress size={18} />
                  </Box>
                </Paper>
              ) : !allPublishedToActivePortals ? (
                <>
                  <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, mb: 2, mt: 3 }}>
                    <ApiPublishForm
                      formData={formData}
                      setFormData={setFormData}
                      showAdvanced={showAdvanced}
                      setShowAdvanced={setShowAdvanced}
                      gateways={gateways}
                      loadingGateways={loadingGateways}
                      newTag={newTag}
                      setNewTag={setNewTag}
                      handleAddTag={handleAddTag}
                      handleRemoveTag={handleRemoveTag}
                      handleCheckboxChange={handleCheckboxChange}
                      handleUrlChange={handleUrlChange}
                    />
                  </Paper>

                  {error && (
                    <Alert severity="error" sx={{ mb: 2 }}>
                      {error}
                    </Alert>
                  )}
                </>
              ) : (
                <Paper variant="outlined" sx={{ p: 3, borderRadius: 2, mb: 2, mt: 3 }}>
                  <Typography variant="h6" fontWeight={600} sx={{ mb: 1 }}>
                    Already Published
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
                    This API is already published to all active developer portals.
                  </Typography>
                </Paper>
              )}

              {!publishedStatusLoading && (
                <Stack direction="row" spacing={1} sx={{ mt: 3 }} justifyContent="space-between">
                  <Button variant="outlined" onClick={() => setActiveStep(0)}>
                    Back
                  </Button>

                  {!allPublishedToActivePortals && (
                    <Button
                      variant="contained"
                      disabled={
                        creating || !selectedPortalId || !formData.apiName || !formData.productionURL || (formData.productionURL.trim() !== '' && !isValidHttpUrl(formData.productionURL))
                      }
                      onClick={async () => {
                        if (!selectedPortalId) return;
                        const payload = buildPublishPayload(formData, selectedPortalId);
                        await handlePublishFromModal(selectedPortalId, payload);
                      }}
                    >
                      {creating ? "Adding..." : "Add to Developer Portal"}
                    </Button>
                  )}
                </Stack>
              )}
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
}) {
  const theme = useTheme();

  const fill = active
    ? theme.palette.success.light
    : completed
      ? theme.palette.success.main
      : theme.palette.grey[100];

  const textColor = completed ? "#fff" : theme.palette.text.primary;
  const ringColor = completed ? "#fff" : theme.palette.text.primary;

  const clipPath = last
    ? "none"
    : "polygon(0% 0%, 100% 0%, 92% 0%, 100% 50%, 92% 100%, 100% 100%, 0% 100%, 0% 0%)";

  const basePadX = 24;
  const contentPadLeft = index === 0 ? basePadX : basePadX + overlap;

  const borderColor = active
    ? theme.palette.grey[300]
    : completed
      ? theme.palette.grey[200]
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
    <DevPortalProvider>
      <ApiPublishProvider>
        <CreateComponentBuildpackProvider>
          <PublishPortalFlowContent onFinish={onFinish} />
        </CreateComponentBuildpackProvider>
      </ApiPublishProvider>
    </DevPortalProvider>
  );
}
