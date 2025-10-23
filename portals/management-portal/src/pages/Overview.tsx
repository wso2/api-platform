// src/pages/Overview.tsx
import React from "react";
import { Box} from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";

import OrgOverview from "./overview/OrgOverview";

import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { slugEquals, slugify } from "../utils/slug";
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

  const currentOrgHandle =
    params.orgHandle ?? organization?.handle ?? "organization";

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

  const handleSelectProject = React.useCallback(
    (project: Project) => {
      setSelectedProject(project);
      navigate(
        `/${currentOrgHandle}/${projectSlugFromName(project.name, project.id)}/overview`
      );
    },
    [currentOrgHandle, navigate, setSelectedProject]
  );

  const handleCreateProject = React.useCallback(
    async (name?: string, description?: string) => {
      try {
        const project = await createProject(name, description);
        await refreshProjects();
        handleSelectProject(project);
      } catch (error) {
        console.error("Failed to create project", error);
      }
    },
    [createProject, refreshProjects, handleSelectProject]
  );

  const showProjectContent = Boolean(selectedProject);

  return (
    <Box>
      {!showProjectContent && (
        <OrgOverview
          projects={projects}
          onSelectProject={handleSelectProject}
          onCreateProject={handleCreateProject}
          // onRefresh={refreshProjects}
          loading={projectsLoading}
        />
      )}

      {/* When a project is selected, the router renders ProjectOverview instead */}
    </Box>
  );
};

export default Overview;
