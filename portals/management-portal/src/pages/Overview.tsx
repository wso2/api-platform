import React from "react";
import { Box } from "@mui/material";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import OrgOverview from "./overview/orgOverview/OrgOverview";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { projectSlugFromName, projectSlugMatches } from "../utils/projectSlug";

import type { Project } from "../hooks/projects";

const Overview: React.FC = () => {
  const navigate = useNavigate();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const {
    projects,
    selectedProject,
    setSelectedProject,
    loading: projectsLoading,
    refreshProjects,
    createProject,
  } = useProjects();
  const [searchParams, setSearchParams] = useSearchParams();

  const currentOrgHandle =
    params.orgHandle ?? organization?.handle ?? "organization";
  const shouldAutoOpenCreate = searchParams.get("createProject") === "true";
  const [autoOpenCreate, setAutoOpenCreate] = React.useState(
    shouldAutoOpenCreate
  );

  React.useEffect(() => {
    const slug = params.projectHandle;

    if (!slug) {
      return;
    }

    const match = projects.find((project) =>
      projectSlugMatches(project.name, project.id, slug)
    );

    if (match) {
      if (!selectedProject || selectedProject.id !== match.id) {
        setSelectedProject(match);
      }
      return;
    }

    if (!projectsLoading) {
      navigate(`/${currentOrgHandle}/overview`, { replace: true });
    }
  }, [
    params.projectHandle,
    projects,
    projectsLoading,
    selectedProject,
    setSelectedProject,
    navigate,
    currentOrgHandle,
  ]);

  React.useEffect(() => {
    if (!shouldAutoOpenCreate) {
      return;
    }
    setAutoOpenCreate(true);
    setSelectedProject(null);
    const next = new URLSearchParams(searchParams);
    next.delete("createProject");
    setSearchParams(next, { replace: true });
  }, [
    searchParams,
    setSearchParams,
    setAutoOpenCreate,
    setSelectedProject,
    shouldAutoOpenCreate,
  ]);

  const handleSelectProject = React.useCallback(
    (project: Project) => {
      setSelectedProject(project);
      navigate(
        `/${currentOrgHandle}/${projectSlugFromName(
          project.name,
          project.id
        )}/overview`
      );
    },
    [currentOrgHandle, navigate, setSelectedProject]
  );

  const handleCreateProject = React.useCallback(
    async (name?: string, description?: string) => {
      try {
        await createProject(name, description);
        await refreshProjects();
        setSelectedProject(null);
        setAutoOpenCreate(false);
        navigate(`/${currentOrgHandle}/overview`, { replace: true });
      } catch (error) {
        console.error("Failed to create project", error);
      }
    },
    [
      createProject,
      currentOrgHandle,
      navigate,
      refreshProjects,
      setAutoOpenCreate,
      setSelectedProject,
    ]
  );

  const showProjectContent = Boolean(selectedProject && params.projectHandle);

  return (
    <Box>
      {!showProjectContent && (
        <OrgOverview
          projects={projects}
          onSelectProject={handleSelectProject}
          onCreateProject={handleCreateProject}
          autoOpenCreate={autoOpenCreate}
          onAutoOpenCreateHandled={() => setAutoOpenCreate(false)}
          // onRefresh={refreshProjects}
          loading={projectsLoading}
        />
      )}

      {/* When a project is selected, the router renders ProjectOverview instead */}
    </Box>
  );
};

export default Overview;
