import React from "react";
import {
  Box,
  Button,
  CircularProgress,
  Divider,
  Stack,
  Typography,
  Alert,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Snackbar,
  Card,
  CardContent,
  IconButton,
  Grid,
  Tooltip,
} from "@mui/material";
import ArrowBackIcon from "@mui/icons-material/ArrowBack";
import LaunchOutlinedIcon from "@mui/icons-material/LaunchOutlined";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { ApiPublishProvider } from "../../context/ApiPublishContext";
import { useApiPublishContext } from "../../hooks/useApiPublishContext";
import { ApiProvider, useApisContext } from "../../context/ApiContext";
import { DevPortalProvider } from "../../context/DevPortalContext";
import { useDevPortals } from "../../hooks/useDevPortals";
import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import { useApiGateways } from "../../hooks/useApiGateways";
import { useApiPublications } from "../../hooks/useApiPublications";
import type { ApiSummary } from "../../hooks/apis";
import type { PortalUIModel } from "../../types/portal";
import type { GatewayUIModel } from "../../hooks/useApiGateways";
import BijiraDPLogo from '../BijiraDPLogo.png'
import ConfirmationDialog from "../../common/ConfirmationDialog";

const PortalPublishCard: React.FC<{
  portal: PortalUIModel;
  isPublished: boolean;
  selection: { sandbox: string; production: string };
  isPublishing: boolean;
  onSelectionChange: (portalId: string, field: 'sandbox' | 'production', value: string) => void;
  onPublish: (portalId: string) => void;
  onUnpublish: (portalId: string) => void;
  apiGateways: GatewayUIModel[];
}> = ({ portal, isPublished, selection, isPublishing, onSelectionChange, onPublish, onUnpublish, apiGateways }) => {
  const logoSrc = portal.logoSrc || BijiraDPLogo;
  const logoAlt = portal.logoAlt || `${portal.name} logo`;
  const selected = selection.sandbox && selection.production;
  const portalUrl = portal.portalUrl;

  return (
    <Grid key={portal.uuid} sx={{ display: 'flex' }}>
      <Card sx={{ 
        maxWidth: 450, 
        p:1, 
        mb:0, 
        borderRadius: 3, 
        boxShadow: '0 0 1px #8d91a3, 0 1px 2px #cbcedb',
        width: '100%',
        display: 'flex',
        flexDirection: 'column'
      }}>
          <CardContent sx={{ p: 2, '&:last-child': { pb: 2 }, mb:0}}>
            <Box
              sx={{
                display: "flex",
                gap: 2,
                alignItems: "start",
              }}
            >
              {/* Logo */}
              <Box
                sx={{
                  width: 100,
                  height: 100,
                  borderRadius: "8px",
                  border: "0.5px solid #abb8c2ff",
                  bgcolor: "#d9e0e4ff",
                  overflow: "hidden",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                }}
              >
                <Box
                  component="img"
                  src={logoSrc}
                  alt={logoAlt}
                  sx={{ width: 90, height: 90, objectFit: "contain" }}
                />
              </Box>

              {/* Title + description + URL */}
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Box sx={{ display: "flex", alignItems: "center" }}>
                  <Typography sx={{ mr: 1 }} fontSize={18} fontWeight={600}>
                    {portal.name}
                  </Typography>
                </Box>

                <Typography
                  sx={{ mt: 0.5, color: "rgba(0,0,0,0.6)", maxWidth: 300 }}
                  variant="body2"
                >
                  {portal.description}
                </Typography>

                <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.5 }}>
                  <Box
                    sx={{
                      flex: 1,
                      minWidth: 0,
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      mr: 0.5
                    }}
                  >
                    <Tooltip
                      title={selected ? "" : "URL not available until published"}
                      placement="top"
                    >
                      <span>
                        <Box
                          component="a"
                          href={selected ? portalUrl : undefined}
                          target="_blank"
                          rel="noopener"
                          sx={{
                            fontWeight: 600,
                            color: selected ? 'inherit' : 'text.disabled',
                            cursor: selected ? 'pointer' : 'not-allowed',
                            textDecoration: 'none',
                            '&:hover': selected ? {
                              textDecoration: 'underline',
                            } : {},
                          }}
                          onClick={(e) => {
                            if (!selected) e.preventDefault();
                            e.stopPropagation();
                          }}
                          title={portalUrl} // Show full URL on hover
                        >
                          {portalUrl}
                        </Box>
                      </span>
                    </Tooltip>
                  </Box>

                  <Tooltip title={selected ? "Open portal URL" : "URL not available until published"} placement="top">
                    <span>
                      <IconButton
                        size="small"
                        sx={{ ml: 0.5 }}
                        disabled={!selected}
                        onClick={(e) => {
                          if (!selected) return;
                          e.stopPropagation();
                          window.open(portalUrl, "_blank", "noopener,noreferrer");
                        }}
                        aria-label="Open portal URL"
                      >
                        <LaunchOutlinedIcon fontSize="inherit" />
                      </IconButton>
                    </span>
                  </Tooltip>
                </Box>
              </Box>
            </Box>

            {/* Divider */}
            <Divider sx={{ my: 2 }} />

            {/* Gateway / Endpoint selectors */}
            <Stack spacing={2}>
              <FormControl fullWidth size="small">
                <InputLabel
                  sx={{
                    color: isPublished ? 'text.primary' : undefined,
                    '&.Mui-disabled': {
                      color: 'text.primary',
                    },
                  }}
                >
                  Sandbox Gateway
                </InputLabel>
                <Select
                  value={selection.sandbox || ""}
                  onChange={(e) =>
                    onSelectionChange(portal.uuid, "sandbox", e.target.value)
                  }
                  label="Sandbox Gateway"
                  disabled={isPublished}
                  sx={{
                    '&.Mui-disabled': {
                      color: 'text.primary',
                      '& .MuiSelect-select': {
                        color: 'text.primary',
                        WebkitTextFillColor: 'currentColor',
                      },
                    },
                  }}
                >
                  {apiGateways.map((gw) => (
                    <MenuItem key={gw.id} value={gw.id}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 1,
                          width: '100%',
                          minWidth: 0, // Allow flex shrinking
                        }}
                      >
                        <Typography
                          variant="body2"
                          sx={{
                            textTransform: 'lowercase',
                            flex: 1,
                            minWidth: 0,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {gw.displayName || gw.name}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{
                            flexShrink: 0,
                            minWidth: 'fit-content',
                          }}
                        >
                          {gw.vhost}
                        </Typography>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <FormControl fullWidth size="small">
                <InputLabel
                  sx={{
                    color: isPublished ? 'text.primary' : undefined,
                    '&.Mui-disabled': {
                      color: 'text.primary',
                    },
                  }}
                >
                  Production Gateway
                </InputLabel>
                <Select
                  value={selection.production || ""}
                  onChange={(e) =>
                    onSelectionChange(portal.uuid, "production", e.target.value)
                  }
                  label="Production Gateway"
                  disabled={isPublished}
                  sx={{
                    '&.Mui-disabled': {
                      color: 'text.primary',
                      '& .MuiSelect-select': {
                        color: 'text.primary',
                        WebkitTextFillColor: 'currentColor',
                      },
                    },
                  }}
                >
                  {apiGateways.map((gw) => (
                    <MenuItem key={gw.id} value={gw.id}>
                      <Box
                        sx={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: 1,
                          width: '100%',
                          minWidth: 0, // Allow flex shrinking
                        }}
                      >
                        <Typography
                          variant="body2"
                          sx={{
                            textTransform: 'lowercase',
                            flex: 1,
                            minWidth: 0,
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {gw.displayName || gw.name}
                        </Typography>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{
                            flexShrink: 0,
                            minWidth: 'fit-content',
                          }}
                        >
                          {gw.vhost}
                        </Typography>
                      </Box>
                    </MenuItem>
                  ))}
                </Select>
              </FormControl>

              <Box sx={{ display: 'flex', gap: 1 }}>
                {isPublished ? (
                  <>
                    <Button
                      variant="contained"
                      fullWidth
                      disabled={true}
                    >
                      Published
                    </Button>
                    <Button
                      variant="outlined"
                      color="error"
                      disabled={isPublishing}
                      onClick={() => onUnpublish(portal.uuid)}
                      sx={{ px: 3 }}
                      startIcon={isPublishing ? <CircularProgress size={16} color="error" /> : undefined}
                    >
                      {isPublishing ? 'Unpublishing...' : 'Unpublish'}
                    </Button>
                  </>
                ) : (
                  <Button
                    variant="contained"
                    fullWidth
                    disabled={isPublishing || !selection.sandbox || !selection.production}
                    onClick={() => onPublish(portal.uuid)}
                    startIcon={isPublishing ? <CircularProgress size={16} /> : undefined}
                  >
                    {isPublishing ? 'Publishing...' : 'Publish'}
                  </Button>
                )}
              </Box>
            </Stack>
          </CardContent>
      </Card>
    </Grid>
  );
};

const ApiPublishContent: React.FC = () => {
  const { orgHandle, projectHandle, apiId: apiIdFromPath } = useParams<{
    orgHandle?: string;
    projectHandle?: string;
    apiId?: string;
  }>();
  const [searchParams] = useSearchParams();
  const navigate = useNavigate();
  const { fetchApiById, loading: apiLoading } = useApisContext();
  const { devportals, loading: portalsLoading, error: portalsError } = useDevPortals();
  const { loading: gatewaysLoading } = useGateways();
  const { publishToDevPortal, unpublishFromDevPortal, getPublishState } = useApiPublishContext();

  // Use new hooks for API-specific data - prioritize route param over query string
  const apiId = apiIdFromPath ?? searchParams.get("apiId") ?? "";
  const { gateways: apiGateways, loading: apiGatewaysLoading, error: apiGatewaysError } = useApiGateways(apiId);
  const { publications, loading: publicationsLoading, error: publicationsError, refetch: refetchPublications } = useApiPublications(apiId);

  // Helper functions for publications
  const isPublishedToPortal = (portalId: string) => {
    const publication = publications.find(p => p.devPortalUuid === portalId);
    return publication?.status === 'published';
  };

  const getPublicationForPortal = (portalId: string) => {
    return publications.find(p => p.devPortalUuid === portalId);
  };

  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [apiError, setApiError] = React.useState<string | null>(null);
  const [selections, setSelections] = React.useState<Record<string, { sandbox: string; production: string }>>({});
  const [publishing, setPublishing] = React.useState<Set<string>>(new Set());
  const [snackbar, setSnackbar] = React.useState<{
    open: boolean;
    message: string;
    severity: 'success' | 'error';
  }>({
    open: false,
    message: '',
    severity: 'success',
  });

  const [confirmationDialog, setConfirmationDialog] = React.useState<{
    open: boolean;
    title: string;
    message: string;
    onConfirm: () => void;
    confirmText?: string;
    severity?: 'info' | 'warning' | 'error';
    portalId?: string;
  }>({
    open: false,
    title: '',
    message: '',
    onConfirm: () => {},
  });


  React.useEffect(() => {
    if (!apiId) return;
    fetchApiById(apiId)
      .then(setApi)
      .catch((err) => setApiError(err.message));
  }, [apiId, fetchApiById]);

  // Initialize selections based on existing publications
  React.useEffect(() => {
    if (publications.length > 0) {
      const initialSelections: Record<string, { sandbox: string; production: string }> = {};
      
      publications.forEach((publication) => {
        if (publication.status === 'published') {
          initialSelections[publication.devPortalUuid] = {
            sandbox: publication.sandboxEndpoint?.gatewayId || '',
            production: publication.productionEndpoint?.gatewayId || '',
          };
        }
      });
      
      setSelections(initialSelections);
    }
  }, [publications]);

  const handleBack = React.useCallback(() => {
    const base =
      orgHandle && projectHandle
        ? `/${orgHandle}/${projectHandle}/apis/${apiId}/overview`
        : orgHandle
        ? `/${orgHandle}/apis/${apiId}/overview`
        : `/apis/${apiId}/overview`;
    navigate(`${base}?apiId=${apiId}`);
  }, [navigate, orgHandle, projectHandle, apiId]);

  const handleSelectionChange = (portalId: string, field: 'sandbox' | 'production', value: string) => {
    setSelections((prev) => ({
      ...prev,
      [portalId]: { ...prev[portalId], [field]: value },
    }));
  };

  const executePublish = async (portalId: string) => {
    if (!apiId) return;
    const selection = selections[portalId];
    if (!selection?.sandbox || !selection?.production) {
      setSnackbar({ open: true, message: 'Please select both sandbox and production gateway endpoints.', severity: 'error' });
      setConfirmationDialog(prev => ({ ...prev, open: false }));
      return;
    }

    const isAlreadyPublished = isPublishedToPortal(portalId);
    const publication = getPublicationForPortal(portalId);

    setPublishing((prev) => new Set(prev).add(portalId));
    try {
      // If already published, unpublish first
      if (isAlreadyPublished && publication) {
        await unpublishFromDevPortal(apiId, publication.uuid);
      }

      // Then publish with new endpoints
      await publishToDevPortal(apiId, portalId, selection.sandbox, selection.production);
      setSnackbar({ open: true, message: `API ${isAlreadyPublished ? 'updated' : 'published'} successfully!`, severity: 'success' });

      // Refetch publications to update the UI
      await refetchPublications();
    } catch (err) {
      setSnackbar({ open: true, message: err instanceof Error ? err.message : 'Failed to publish', severity: 'error' });
    } finally {
      setPublishing((prev) => {
        const next = new Set(prev);
        next.delete(portalId);
        return next;
      });
      setConfirmationDialog(prev => ({ ...prev, open: false }));
    }
  };  const executeUnpublish = async (portalId: string) => {
    if (!apiId) return;
    const publication = getPublicationForPortal(portalId);
    if (!publication) {
      setConfirmationDialog(prev => ({ ...prev, open: false }));
      return;
    }

    setPublishing((prev) => new Set(prev).add(portalId));
    try {
      await unpublishFromDevPortal(apiId, publication.uuid);
      setSnackbar({ open: true, message: 'API unpublished successfully!', severity: 'success' });

      // Refetch publications to update the UI
      await refetchPublications();
    } catch (err) {
      setSnackbar({ open: true, message: err instanceof Error ? err.message : 'Failed to unpublish', severity: 'error' });
    } finally {
      setPublishing((prev) => {
        const next = new Set(prev);
        next.delete(portalId);
        return next;
      });
      setConfirmationDialog(prev => ({ ...prev, open: false }));
    }
  };

  const handlePublish = (portalId: string) => {
    const portal = devportals.find(p => p.uuid === portalId);
    const portalName = portal?.name || 'this portal';

    setConfirmationDialog({
      open: true,
      title: 'Publish API',
      message: `Are you sure you want to publish this API to ${portalName}? This will make the API available to developers through the portal.`,
      onConfirm: () => executePublish(portalId),
      confirmText: 'Publish',
      severity: 'info',
      portalId,
    });
  };

  const handleUnpublish = (portalId: string) => {
    const portal = devportals.find(p => p.uuid === portalId);
    const portalName = portal?.name || 'this portal';

    setConfirmationDialog({
      open: true,
      title: 'Unpublish API',
      message: `Are you sure you want to unpublish this API from ${portalName}? This action cannot be undone and will remove the API from the developer portal.`,
      onConfirm: () => executeUnpublish(portalId),
      confirmText: 'Unpublish',
      severity: 'error',
      portalId,
    });
  };

  if (apiLoading || portalsLoading || gatewaysLoading || apiGatewaysLoading || publicationsLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  if (apiError || portalsError || apiGatewaysError || publicationsError) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700} gutterBottom>
          Error
        </Typography>
        <Typography color="error">{String(apiError || portalsError || apiGatewaysError || publicationsError)}</Typography>
        <Button
          variant="contained"
          startIcon={<ArrowBackIcon />}
          onClick={handleBack}
          sx={{ textTransform: "none", mt: 2 }}
        >
          Back
        </Button>
      </Box>
    );
  }

  if (!api) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700} gutterBottom>
          API not found
        </Typography>
        <Button
          variant="contained"
          startIcon={<ArrowBackIcon />}
          onClick={handleBack}
          sx={{ textTransform: "none", mt: 2 }}
        >
          Back
        </Button>
      </Box>
    );
  }

  return (
    <Box sx={{ overflowX: "auto" }}>
      {/* Header */}
      <Stack direction="row" alignItems="center" spacing={2} mb={3}>
        <Button
          startIcon={<ArrowBackIcon />}
          onClick={handleBack}
          sx={{ textTransform: "none" }}
        >
          Back
        </Button>
        <Typography variant="h5" fontWeight={700}>
          Publish {api.name} to DevPortal
        </Typography>
      </Stack>

      <Grid container spacing={2} ml={1} mb={1}>
      {devportals.filter((portal) => portal.isActive).map((portal) => {
  const publishState = getPublishState(apiId);
  const isPublished = isPublishedToPortal(portal.uuid) || publishState?.apiPortalRefId === portal.uuid;
  const selection = selections[portal.uuid] || { sandbox: '', production: '' };
  const isPublishing = publishing.has(portal.uuid);

        return (
          <PortalPublishCard
            key={portal.uuid}
            portal={portal}
            isPublished={isPublished}
            selection={selection}
            isPublishing={isPublishing}
            onSelectionChange={handleSelectionChange}
            onPublish={handlePublish}
            onUnpublish={handleUnpublish}
            apiGateways={apiGateways}
          />
        );
      })}

      {/* Confirmation Dialog */}
      <ConfirmationDialog
        open={confirmationDialog.open}
        onClose={() => setConfirmationDialog((prev) => ({ ...prev, open: false }))}
        onConfirm={() => {
          // start the configured action (parent executes and manages publishing state)
          void confirmationDialog.onConfirm();
        }}
        title={confirmationDialog.title}
        message={confirmationDialog.message}
        confirmText={confirmationDialog.confirmText}
        severity={confirmationDialog.severity}
      />

      {/* Snackbar */}
      <Snackbar
        open={snackbar.open}
        autoHideDuration={4000}
        onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      >
        <Alert
          onClose={() => setSnackbar((prev) => ({ ...prev, open: false }))}
          severity={snackbar.severity}
          sx={{ width: "100%" }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>

      {/* second confirmation dialog removed (duplicate) */}
    </Grid>
    </Box>
  );
};

const ApiPublish: React.FC = () => (
  <ApiPublishProvider>
    <ApiProvider>
      <DevPortalProvider>
        <GatewayProvider>
          <ApiPublishContent />
        </GatewayProvider>
      </DevPortalProvider>
    </ApiProvider>
  </ApiPublishProvider>
);

export default ApiPublish;