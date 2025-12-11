import React from "react";
import { Box, CircularProgress, Stack, Typography } from "@mui/material";
import AddIcon from "@mui/icons-material/Add";
import { useNavigate, useParams } from "react-router-dom";
import { Button } from "../components/src/components/Button";
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
import NoDataImage from "../pages/overview/widgets/NoData.svg";

import ApiCard, { type ApiCardData } from "../pages/apis/ApiCard";

const toCardData = (api: ApiSummary): ApiCardData => {
  const transports = api.transport ?? [];
  const tagList = transports.slice(0, 2);
  const extraTagsCount = transports.length > 2 ? transports.length - 2 : 0;

  return {
    id: api.id,
    name: api.displayName ?? api.name,
    provider: api.provider ?? "Unknown",
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
  const { projects, selectedProject, setSelectedProject, projectsLoaded } =
    useProjects();
  const {
    apis,
    loading,
    createApi,
    selectApi,
    deleteApi,
    importOpenApi,
    refreshApis,
  } = useApisContext();

  const [query, setQuery] = React.useState("");
  const [wizardOpen, setWizardOpen] = React.useState(false);
  const [templatesOpen, setTemplatesOpen] = React.useState(false);
  const [contractOpen, setContractOpen] = React.useState(false);
  const [deleteDialog, setDeleteDialog] = React.useState<{
    open: boolean;
    apiId: string;
    apiDisplayName: string;
  }>({ open: false, apiId: "", apiDisplayName: "" });

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
    (apiId: string, apiDisplayName: string, e: React.MouseEvent) => {
      e.stopPropagation();
      setDeleteDialog({ open: true, apiId, apiDisplayName });
    },
    []
  );

  const confirmDelete = React.useCallback(async () => {
    try {
      await deleteApi(deleteDialog.apiId);
      setDeleteDialog({ open: false, apiId: "", apiDisplayName: "" });
    } catch (error) {
      console.error("Failed to delete API:", error);
    }
  }, [deleteApi, deleteDialog.apiId]);

  const isFirstTimeEmpty =
    !loading && !query.trim() && filteredApis.length === 0;

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

      {templatesOpen && !wizardOpen && !contractOpen && (
        <ApiEmptyState onAction={handleEmptyStateAction} />
      )}

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
            // Search active but no results: show toolbar + message in same grid
            <Box
              sx={{
                display: "grid",
                gap: 3,
                justifyContent: "center",
                gridTemplateColumns: "repeat(auto-fill, 320px)",
              }}
            >
              {/* Toolbar */}
              <Box
                sx={{
                  gridColumn: "1 / -1",
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  mb: 1,
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
              <Box
                sx={{
                  gridColumn: "1 / -1",
                  textAlign: "center",
                  mt: 4,
                }}
              >
                {/* No Data Illustration */}
                <Box
                  component="img"
                  src={NoDataImage}
                  alt="No data"
                  sx={{
                    width: 60,
                    height: "auto",
                    opacity: 0.9,
                    mb: 1,
                  }}
                />

                <Typography variant="h5" fontWeight={700}>
                  No APIs match “{query}”
                </Typography>
                <Typography color="text.secondary" sx={{ mt: 1 }}>
                  Try a different name, context, or tag.
                </Typography>
              </Box>
            </Box>
          ) : isFirstTimeEmpty ? (
            <ApiEmptyState onAction={handleEmptyStateAction} />
          ) : (
            // Normal view: toolbar + cards grid
            <Box
              sx={{
                display: "grid",
                gap: 3,
                justifyContent: "center",
                gridTemplateColumns: "repeat(auto-fill, 320px)",
              }}
            >
              {/* Toolbar */}
              <Box
                sx={{
                  gridColumn: "1 / -1",
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                  mb: 1,
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

              {/* Cards */}
              {filteredApis.map((apiSummary) => {
                const card = toCardData(apiSummary);
                return (
                  <ApiCard
                    key={apiSummary.id}
                    api={card}
                    onClick={() => handleNavigate(apiSummary)}
                    onDelete={(e) =>
                      handleDelete(
                        apiSummary.id,
                        apiSummary.displayName ?? apiSummary.name,
                        e
                      )
                    }
                  />
                );
              })}
            </Box>
          )}
        </>
      )}

      <ConfirmationDialog
        open={deleteDialog.open}
        onClose={() =>
          setDeleteDialog({ open: false, apiId: "", apiDisplayName: "" })
        }
        onConfirm={confirmDelete}
        title="Delete API"
        message={
          <Box>
            <Typography
              variant="body1"
              sx={{ display: "inline", mr: 0.5, color: "text.primary" }}
            >
              Are you sure you want to remove the API
            </Typography>
            <Typography
              component="span"
              sx={{ fontWeight: 700, fontSize: "1.05rem", ml: 0.5 }}
            >
              "{deleteDialog.apiDisplayName}"
            </Typography>
            <Typography
              variant="body1"
              sx={{ ml: 0.5, display: "inline", color: "text.primary" }}
            >
              ?
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
              This action will be irreversible and all related details will be
              lost. Please type the API display name below to confirm.
            </Typography>
          </Box>
        }
        primaryBtnText="Delete"
        cancelText="Cancel"
        severity="error"
        confirmationText={deleteDialog.apiDisplayName}
        confirmationPlaceholder={"Enter API display name to confirm"}
      />
    </Box>
  );
};

const APIs: React.FC = () => <ApiListContent />;

export default APIs;
