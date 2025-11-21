import React from "react";
import {
  Box,
  CardActionArea,
  CardContent,
  Chip,
  CircularProgress,
  Grid,
  Rating,
  Stack,
  Tooltip,
  Typography,
} from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import DeleteIcon from "@mui/icons-material/Delete";
import { useNavigate, useParams } from "react-router-dom";

import { Card } from "../components/src/components/Card";
import { Button } from "../components/src/components/Button";
import { IconButton } from "../components/src/components/IconButton";
import theme from "../theme";
import { openSidebarGroup } from "../utils/sidebar";
import { useApisContext } from "../context/ApiContext";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { slugify } from "../utils/slug";
import { projectSlugFromName, projectSlugMatches } from "../utils/projectSlug";
import type { ApiSummary } from "../hooks/apis";
import ApiEmptyState, {
  type EmptyStateAction,
} from "../components/apis/ApiEmptyState";
import { SearchBar } from "../components/src/components/SearchBar";
import EndPointCreationFlow from "./apis/CreationFlows/EndPointCreationFlow";
import APIContractCreationFlow from "./apis/CreationFlows/APIContractCreationFlow";
import ConfirmationDialog from "../common/ConfirmationDialog";

/* ---------------- helpers ---------------- */

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
  onDelete: (e: React.MouseEvent) => void;
}> = ({ api, onClick, onDelete }) => (
  <Card
    style={{
      padding: 16,
      position: "relative",
      maxWidth: 350,
      minWidth: 300,
      minHeight: 260,
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
            <Typography variant="h4" sx={{ lineHeight: 1.2 }}>
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

        {(() => {
          const desc = api.description ?? "";
          const isTruncated = desc.length > 90;
          const short = isTruncated ? desc.slice(0, 90).trimEnd() + "…" : desc;

          return isTruncated ? (
            <Tooltip title={desc} placement="top" arrow>
              <Typography
                fontSize={12}
                color="#bbbabaff"
                sx={{ mb: 1.25 }}
              >
                {short}
              </Typography>
            </Tooltip>
          ) : (
            <Typography fontSize={12} color="#bbbabaff" sx={{ mb: 1.25 }}>
              {desc}
            </Typography>
          );
        })()}

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
    <IconButton
      onClick={onDelete}
      size="small"
      color="error"
      aria-label="Delete API"
      sx={{
        position: "absolute",
        bottom: 8,
        right: 8,
      }}
    >
      <DeleteIcon fontSize="small" />
    </IconButton>
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

/* ---------------- page ---------------- */

const ApiListContent: React.FC = () => {
  const navigate = useNavigate();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const { projects, selectedProject, setSelectedProject, projectsLoaded } =
    useProjects();
  const { apis, loading, createApi, selectApi, deleteApi, importOpenApi, refreshApis } = useApisContext();

  const [query, setQuery] = React.useState("");
  const [wizardOpen, setWizardOpen] = React.useState(false);
  const [templatesOpen, setTemplatesOpen] = React.useState(false);
  const [contractOpen, setContractOpen] = React.useState(false);
  const [deleteDialog, setDeleteDialog] = React.useState<{
    open: boolean;
    apiId: string;
    apiName: string;
  }>({ open: false, apiId: "", apiName: "" });

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
    const match = projects.find((p) =>
      projectSlugMatches(p.name, p.id, projectSlugParam)
    );
    if (match) setSelectedProject(match);
  }, [
    projectSlugParam,
    projects,
    projectsLoaded,
    selectedProject,
    setSelectedProject,
  ]);

  React.useEffect(() => {
    selectApi(null);
  }, [selectApi]);

  const filteredApis = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return apis;
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
      selectApi(apiSummary, { slug: apiSlug });

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
    [navigate, orgHandle, projectSlug, selectApi]
  );

  const handleDelete = React.useCallback(
    (apiId: string, apiName: string, e: React.MouseEvent) => {
      e.stopPropagation();
      setDeleteDialog({ open: true, apiId, apiName });
    },
    []
  );

  const confirmDelete = React.useCallback(async () => {
    try {
      await deleteApi(deleteDialog.apiId);
      setDeleteDialog({ open: false, apiId: "", apiName: "" });
    } catch (error) {
      console.error("Failed to delete API:", error);
    }
  }, [deleteApi, deleteDialog.apiId]);

  // First-time empty (no search & no apis) -> hide toolbar
  const isFirstTimeEmpty =
    !loading && !query.trim() && filteredApis.length === 0;

  // Show toolbar when not inside flows/templates and not first-time empty
  const showToolbar =
    !wizardOpen && !templatesOpen && !contractOpen && !isFirstTimeEmpty;

  const handleEmptyStateAction = React.useCallback(
    (action: EmptyStateAction) => {
      if (action.type === "createFromEndpoint") {
        setTemplatesOpen(false);
        setWizardOpen(true);
        return;
      }
      if (action.type === "learnMore" && action.template === "contract") {
        setTemplatesOpen(false);
        setContractOpen(true);
        return;
      }
      // other templates can be wired similarly
      console.info("Learn more clicked", action.template);
    },
    []
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
      {/* Toolbar (top of page, hidden during first-time empty or flows/templates) */}
      {showToolbar && (
        <Box
          sx={{
            mb: 2,
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 2,
          }}
        >
          <Typography variant="h3">APIs</Typography>

          <Stack direction="row" spacing={1} alignItems="center">
            <SearchBar
              testId="api-search"
              placeholder="Search APIs"
              inputValue={query}
              onChange={setQuery}
              iconPlacement="left"
              bordered
              size="medium"
              color="secondary"
            />
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              sx={{ textTransform: "none" }}
              onClick={() => setTemplatesOpen(true)}
            >
              Create
            </Button>
          </Stack>
        </Box>
      )}

      {/* Endpoint Creation Flow */}
      {wizardOpen && (
        <EndPointCreationFlow
          open={wizardOpen}
          selectedProjectId={selectedProject.id}
          createApi={createApi}
          onClose={() => {
            setWizardOpen(false);
            setTemplatesOpen(false);
          }}
        />
      )}

      {/* Contract Creation Flow */}
      {contractOpen && (
        <APIContractCreationFlow
          open={contractOpen}
          selectedProjectId={selectedProject.id}
          importOpenApi={importOpenApi}
          refreshApis={refreshApis}
          onClose={() => {
            setContractOpen(false);
            setTemplatesOpen(false);
          }}
        />
      )}

      {/* Template grid (visible when user presses Create) */}
      {templatesOpen && !wizardOpen && !contractOpen && (
        <ApiEmptyState onAction={handleEmptyStateAction} />
      )}

      {/* Main content area */}
      {!wizardOpen && !templatesOpen && !contractOpen && (
        <>
          {loading ? (
            <Box
              display="flex"
              alignItems="center"
              justifyContent="center"
              mt={6}
            >
              <CircularProgress size={28} />
            </Box>
          ) : query.trim() && filteredApis.length === 0 ? (
            // Search active but no matches: show message below the toolbar
            <Box textAlign="center" mt={6}>
              <Typography variant="h5" fontWeight={700}>
                No APIs match “{query}”
              </Typography>
              <Typography color="text.secondary" sx={{ mt: 1 }}>
                Try a different name, context, or tag.
              </Typography>
            </Box>
          ) : isFirstTimeEmpty ? (
            // First-time empty state (no search) — toolbar hidden above
            <ApiEmptyState onAction={handleEmptyStateAction} />
          ) : (
            // List of APIs
            <Grid container spacing={2} alignItems="stretch">
              {filteredApis.map((apiSummary) => {
                const card = toCardData(apiSummary);
                return (
                  <Grid
                    size={{ xs: 12, sm: 6, md: 4, lg: 3, xl: 3 }}
                    key={apiSummary.id}
                  >
                    <ApiCard
                      api={card}
                      onClick={() => handleNavigate(apiSummary)}
                      onDelete={(e) => handleDelete(apiSummary.id, apiSummary.name, e)}
                    />
                  </Grid>
                );
              })}
            </Grid>
          )}
        </>
      )}

      <ConfirmationDialog
        open={deleteDialog.open}
        onClose={() => setDeleteDialog({ open: false, apiId: "", apiName: "" })}
        onConfirm={confirmDelete}
        title="Delete API"
        message={`Are you sure you want to delete "${deleteDialog.apiName}"? This action cannot be undone.`}
        confirmText="Delete"
        cancelText="Cancel"
        severity="error"
        confirmationText={deleteDialog.apiName}
        confirmationLabel={`Type the API name to confirm deletion`}
      />
    </Box>
  );
};

const APIs: React.FC = () => <ApiListContent />;

export default APIs;
