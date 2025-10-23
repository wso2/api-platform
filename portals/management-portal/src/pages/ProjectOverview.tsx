import React from "react";
import { Box, Typography } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { slugEquals } from "../utils/slug";
import { projectSlugMatches } from "../utils/projectSlug";

const ProjectOverview: React.FC = () => {
  const navigate = useNavigate();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization } = useOrganization();
  const {
    projects,
    selectedProject,
    setSelectedProject,
    loading,
    projectsLoaded,
  } =
    useProjects();

  const orgHandle = params.orgHandle ?? organization?.handle ?? "";
  const projectSlug = params.projectHandle ?? "";

  const resolvedProject = React.useMemo(() => {
    if (selectedProject) {
      return selectedProject;
    }
    if (!projectSlug) {
      return null;
    }
    return projects.find((project) =>
      projectSlugMatches(project.name, project.id, projectSlug)
    );
  }, [projects, selectedProject, projectSlug]);

  React.useEffect(() => {
    if (!projectSlug || loading || !projectsLoaded) {
      return;
    }
    if (resolvedProject) {
      if (!selectedProject || selectedProject.id !== resolvedProject.id) {
        setSelectedProject(resolvedProject);
      }
      return;
    }
    if (!loading && orgHandle) {
      navigate(`/${orgHandle}/overview`, { replace: true });
    }
  }, [
    projectSlug,
    resolvedProject,
    selectedProject,
    setSelectedProject,
    loading,
    projectsLoaded,
    orgHandle,
    navigate,
  ]);

  if (loading) {
    return null;
  }

  if (!projectsLoaded) {
    return null;
  }

  if (!resolvedProject) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700}>
          Select a project to continue.
        </Typography>
      </Box>
    );
  }

  const displayName = resolvedProject.name;

  return (
    <Box textAlign="center" mt={3}>
      <Typography variant="h4" fontWeight={700} gutterBottom>
        Welcome to {displayName}
      </Typography>
      <Typography color="text.secondary">
        Manage all aspects of the {displayName} project from here.
      </Typography>
    </Box>
  );
};

export default ProjectOverview;
