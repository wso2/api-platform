// portals/management-portal/src/pages/ProjectOverview.tsx
import React from "react";
import { Box, Typography } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { projectSlugMatches } from "../utils/projectSlug";

// ⬇️ import the banner
import HeroBanner, { type BannerSlide } from "../pages/HeroBanner";

// ⬇️ import your SVG images
import deepWorkImg from "./undraw_deep-work_muov.svg";
import typingCodeImg from "./undraw_typing-code_6t2b.svg";

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
  } = useProjects();

  const orgHandle = params.orgHandle ?? organization?.handle ?? "";
  const projectSlug = params.projectHandle ?? "";

  const resolvedProject = React.useMemo(() => {
    if (selectedProject) return selectedProject;
    if (!projectSlug) return null;
    return projects.find((project) =>
      projectSlugMatches(project.name, project.id, projectSlug)
    );
  }, [projects, selectedProject, projectSlug]);

  React.useEffect(() => {
    if (!projectSlug || loading || !projectsLoaded) return;
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

  if (loading || !projectsLoaded) return null;

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

  // Slides for the HeroBanner (now with images)
  const slides: BannerSlide[] = [
    {
      id: "apis",
      tag: "Getting started",
      title: "Create your first API",
      subtitle:
        "Point us at your backend endpoint and we’ll scaffold an API with routing, context, and versioning.",
      ctaLabel: "Create API",
      onCtaClick: () => navigate(`/${orgHandle}/${projectSlug}/apis`),
      imageUrl: typingCodeImg, // ← added image
    },
    {
      id: "gateways",
      tag: "Traffic & Security",
      title: "Set up a Gateway",
      subtitle:
        "Add rate limits, auth, and observability to protect and monitor your services.",
      ctaLabel: "Configure Gateway",
      onCtaClick: () => navigate(`/${orgHandle}/${projectSlug}/gateway`),
      imageUrl: deepWorkImg, // ← added image
    },
  ];

  return (
    <>
      <Box textAlign="center" mt={3}>
        <Typography variant="h4" fontWeight={700}>
          Welcome to {displayName}
        </Typography>
        <Typography color="text.secondary">
          Manage all aspects of the {displayName} project from here.
        </Typography>
      </Box>

      <Box mt={3} pr={6} pl={6}>
        <HeroBanner slides={slides} height={180} intervalMs={5000} />
      </Box>
    </>
  );
};

export default ProjectOverview;
