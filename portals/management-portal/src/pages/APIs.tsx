// src/pages/APIs.tsx
import React from "react";
import {
  Alert,
  Box,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  IconButton,
  InputAdornment,
  Rating,
  Stack,
  TextField,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import SearchIcon from "@mui/icons-material/Search";
import { useNavigate, useParams } from "react-router-dom";

import { Card } from "../components/src/components/Card";
import { Button } from "../components/src/components/Button";
import theme from "../theme";
import { openSidebarGroup } from "../utils/sidebar";
import { ApiProvider, useApisContext } from "../context/ApiContext";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { slugEquals, slugify } from "../utils/slug";
import { projectSlugFromName, projectSlugMatches } from "../utils/projectSlug";
import type { ApiSummary } from "../hooks/apis";
import ApiEmptyState, {
  type EmptyStateAction,
} from "../components/apis/ApiEmptyState";
import EndpointCreationDialog, {
  type EndpointCreationState,
  type EndpointWizardStep,
} from "../components/apis/EndpointCreationDialog";

type ApiCardData = {
  id: string;
  name: string;
  owner: string;
  version: string;
  context: string;
  description: string;
  tags: string[];
  extraTagsCount: number;
  rating: number;
};

const initials = (name: string) => {
  const letters = name.replace(/[^A-Za-z]/g, "");
  if (!letters) return "API";
  return (letters[0] + (letters[1] || "")).toUpperCase();
};

const ApiCard: React.FC<{
  api: ApiCardData;
  onClick: () => void;
}> = ({ api, onClick }) => (
  <Card
    style={{
      padding: 16,
      position: "relative",
      maxWidth: 350,
      minWidth: 300,
    }}
    testId={api.id}
  >
    <CardActionArea onClick={onClick} sx={{ borderRadius: 1 }}>
      <CardContent sx={{ p: 0 }}>
        <Box display="flex" alignItems="center" gap={2}>
          <Box
            sx={{
              width: 65,
              height: 65,
              borderRadius: 0.5,
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
              mb: 1.5,
            }}
          >
            {initials(api.name)}
          </Box>
          <Box>
            <Typography variant="h6" sx={{ lineHeight: 1.2 }}>
              {api.name}
            </Typography>
            <Stack direction="row" spacing={2} useFlexGap>
              <Box>
                <Typography variant="caption" color="text.secondary">
                  By: {api.owner}
                </Typography>
              </Box>
            </Stack>
          </Box>
        </Box>

        <Stack direction="row" spacing={3} sx={{ mb: 1 }}>
          <Box>
            <Typography variant="caption" color="text.secondary">
              Version
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {api.version}
            </Typography>
          </Box>
          <Box>
            <Typography variant="caption" color="text.secondary">
              Context
            </Typography>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {api.context}
            </Typography>
          </Box>
        </Stack>

        <Typography fontSize={12} color="text.secondary" sx={{ mb: 1.25 }}>
          {api.description}
        </Typography>

        <Stack direction="row" spacing={1} sx={{ flexWrap: "wrap", mb: 1 }}>
          {api.tags.map((t) => (
            <Chip
              key={t}
              label={t}
              size="small"
              variant="outlined"
              sx={{ borderRadius: 1 }}
            />
          ))}
          {!!api.extraTagsCount && (
            <Chip
              label={`+${api.extraTagsCount}`}
              size="small"
              variant="outlined"
              sx={{ borderRadius: 1 }}
            />
          )}
        </Stack>

        <Stack direction="row" alignItems="center" spacing={1}>
          <Rating size="small" readOnly value={api.rating} precision={0.5} />
          <Typography variant="caption" color="text.secondary">
            {api.rating.toFixed(1)}/5.0
          </Typography>
        </Stack>
      </CardContent>
    </CardActionArea>
  </Card>
);

const toCardData = (api: ApiSummary): ApiCardData => {
  const transports = api.transport ?? [];
  const tagList = transports.slice(0, 2);
  const extraTagsCount = transports.length > 2 ? transports.length - 2 : 0;

  return {
    id: api.id,
    name: api.name,
    owner: api.provider ?? "Unknown",
    version: api.version,
    context: api.context,
    description: api.description ?? "No description provided.",
    tags: tagList,
    extraTagsCount,
    rating: 4.0,
  };
};

const ApiListContent: React.FC = () => {
  const navigate = useNavigate();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const {
    projects,
    selectedProject,
    setSelectedProject,
    projectsLoaded,
  } = useProjects();
  const { apis, loading, error, createApi } = useApisContext();

  const [query, setQuery] = React.useState("");
  const [wizardOpen, setWizardOpen] = React.useState(false);
  const [wizardStep, setWizardStep] =
    React.useState<EndpointWizardStep>("endpoint");
  const [wizardState, setWizardState] = React.useState<
    EndpointCreationState & { contextEdited: boolean }
  >({
    endpointUrl: "",
    name: "",
    context: "",
    version: "1.0.0",
    description: "",
    contextEdited: false,
  });
  const [wizardError, setWizardError] = React.useState<string | null>(null);
  const [creating, setCreating] = React.useState(false);

  const orgHandle = organization?.handle ?? params.orgHandle ?? "";
  const projectSlugParam = params.projectHandle ?? null;
  const projectSlug = React.useMemo(() => {
    if (selectedProject) {
      return projectSlugFromName(selectedProject.name, selectedProject.id);
    }
    return projectSlugParam;
  }, [selectedProject, projectSlugParam]);

  React.useEffect(() => {
    if (selectedProject || !projectsLoaded || !projectSlugParam) {
      return;
    }

    const match = projects.find((project) =>
      projectSlugMatches(project.name, project.id, projectSlugParam)
    );

    if (match) {
      setSelectedProject(match);
    }
  }, [
    projectSlugParam,
    projects,
    projectsLoaded,
    selectedProject,
    setSelectedProject,
  ]);

  const filteredApis = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      return apis;
    }
    return apis.filter((api) => {
      const transports = api.transport ?? [];
      return (
        api.name.toLowerCase().includes(q) ||
        api.context.toLowerCase().includes(q) ||
        transports.some((tag) => tag.toLowerCase().includes(q))
      );
    });
  }, [apis, query]);

  const handleNavigate = React.useCallback(
    (apiSummary: ApiSummary) => {
      openSidebarGroup("apis");
      const apiSlug = slugify(apiSummary.name);
      const search = new URLSearchParams({ apiId: apiSummary.id }).toString();

      if (orgHandle && projectSlug) {
        navigate(
          `/${orgHandle}/${projectSlug}/${apiSlug}/apioverview?${search}`
        );
        return;
      }

      if (orgHandle) {
        navigate(`/${orgHandle}/${apiSlug}/apioverview?${search}`);
        return;
      }

      navigate(`/${apiSlug}/apioverview?${search}`);
    },
    [navigate, orgHandle, projectSlug]
  );

  const resetWizard = React.useCallback(() => {
    setWizardStep("endpoint");
    setWizardState({
      endpointUrl: "",
      name: "",
      context: "",
      version: "1.0.0",
      description: "",
      contextEdited: false,
    });
    setWizardError(null);
  }, []);

  const handleEmptyStateAction = React.useCallback(
    (action: EmptyStateAction) => {
      if (action.type === "createFromEndpoint") {
        resetWizard();
        setWizardOpen(true);
        return;
      }
      console.info("Learn more clicked", action.template);
    },
    [resetWizard]
  );

  const handleWizardStateChange = React.useCallback(
    (patch: Partial<EndpointCreationState & { contextEdited?: boolean }>) => {
      setWizardState((prev) => ({
        ...prev,
        ...patch,
      }));
    },
    []
  );

  const handleNameChange = React.useCallback((value: string) => {
    setWizardState((prev) => {
      const next = { ...prev, name: value };
      if (!prev.contextEdited) {
        const slug = value
          .trim()
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, "-")
          .replace(/^-+|-+$/g, "");
        next.context = slug ? `/${slug}` : "";
      }
      return next;
    });
  }, []);

  const handleContextChange = React.useCallback((value: string) => {
    handleWizardStateChange({ context: value, contextEdited: true });
  }, [handleWizardStateChange]);

  const handleWizardCreate = React.useCallback(async () => {
    if (!selectedProject) {
      return;
    }

    const endpointUrl = wizardState.endpointUrl.trim();
    const name = wizardState.name.trim();
    const context = wizardState.context.trim();
    const version = wizardState.version.trim() || "1.0.0";

    if (!endpointUrl || !name || !context) {
      setWizardError("Please complete all required fields.");
      return;
    }

    try {
      setWizardError(null);
      setCreating(true);
      const uniqueBackendName = `default-backend-${Date.now().toString(36)}${Math.random()
        .toString(36)
        .slice(2, 8)}`;

      await createApi({
        name,
        context: context.startsWith("/") ? context : `/${context}`,
        version,
        description: wizardState.description?.trim() || undefined,
        projectId: selectedProject.id,
        backendServices: [
          {
            name: uniqueBackendName,
            isDefault: true,
            endpoints: [
              {
                url: endpointUrl,
                description: "Default backend endpoint",
              },
            ],
            retries: 0,
          },
        ],
      });
      setWizardOpen(false);
      resetWizard();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create API";
      setWizardError(message);
    } finally {
      setCreating(false);
    }
  }, [createApi, resetWizard, selectedProject, wizardState]);

  const inferNameFromEndpoint = React.useCallback((url: string) => {
    try {
      const withoutQuery = url.split("?")[0];
      const segments = withoutQuery.split("/").filter(Boolean);
      const candidate = segments[segments.length - 1] ?? "api";
      const clean = candidate.replace(/[^a-zA-Z0-9]+/g, " ").trim();
      if (!clean) return "Sample API";
      return clean
        .split(" ")
        .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
        .join(" ");
    } catch {
      return "Sample API";
    }
  }, []);

  const handleWizardStepChange = React.useCallback(
    (nextStep: EndpointWizardStep) => {
      if (nextStep === "details") {
        const inferredName = inferNameFromEndpoint(wizardState.endpointUrl);
        if (!wizardState.name.trim()) {
          handleNameChange(inferredName);
        }
        if (!wizardState.contextEdited && !wizardState.context.trim()) {
          const slug = inferredName
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, "-")
            .replace(/^-+|-+$/g, "");
          handleWizardStateChange({ context: slug ? `/${slug}` : "", contextEdited: false });
        }
      }
      setWizardStep(nextStep);
    },
    [handleNameChange, handleWizardStateChange, inferNameFromEndpoint, wizardState]
  );

  const dialogState = React.useMemo<EndpointCreationState>(
    () => ({
      endpointUrl: wizardState.endpointUrl,
      name: wizardState.name,
      context: wizardState.context,
      version: wizardState.version,
      description: wizardState.description,
    }),
    [wizardState]
  );

  if (!projectsLoaded) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={28} />
      </Box>
    );
  }

  if (!selectedProject) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700}>
          Select a project to view APIs.
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      <Box
        sx={{
          mb: 2,
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 2,
        }}
      >
        <Typography variant="h5">APIs</Typography>

        <Stack direction="row" spacing={1} alignItems="center">
          <TextField
            size="small"
            placeholder="Search"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <IconButton edge="start" disableRipple tabIndex={-1}>
                    <SearchIcon fontSize="small" />
                  </IconButton>
                </InputAdornment>
              ),
            }}
          />
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            sx={{ textTransform: "none" }}
            onClick={() => {
              resetWizard();
              setWizardOpen(true);
            }}
          >
            Create
          </Button>
        </Stack>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading ? (
        <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
          <CircularProgress size={28} />
        </Box>
      ) : filteredApis.length === 0 ? (
        <ApiEmptyState onAction={handleEmptyStateAction} />
      ) : (
        <Box
          sx={{
            display: "flex",
            gap: 2,
            alignItems:"flex-start"
          }}
        >
          {filteredApis.map((apiSummary) => {
            const card = toCardData(apiSummary);
            return (
              <ApiCard
                key={apiSummary.id}
                api={card}
                onClick={() => handleNavigate(apiSummary)}
              />
            );
          })}
        </Box>
      )}

      <EndpointCreationDialog
        open={wizardOpen}
        step={wizardStep}
        state={dialogState}
        onChange={(patch) => {
          if (Object.prototype.hasOwnProperty.call(patch, "name")) {
            handleNameChange(patch.name ?? "");
            return;
          }
          if (Object.prototype.hasOwnProperty.call(patch, "context")) {
            handleContextChange(patch.context ?? "");
            return;
          }
          handleWizardStateChange(patch);
        }}
        onStepChange={handleWizardStepChange}
        onClose={() => {
          if (!creating) {
            setWizardOpen(false);
            resetWizard();
          }
        }}
        onCreate={handleWizardCreate}
        creating={creating}
      />

      {wizardOpen && wizardError && (
        <Alert severity="error" sx={{ mt: 2 }}>
          {wizardError}
        </Alert>
      )}
    </Box>
  );
};

const APIs: React.FC = () => (
  <ApiProvider>
    <ApiListContent />
  </ApiProvider>
);

export default APIs;
