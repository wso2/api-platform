// portals/management-portal/src/pages/ProjectOverview.tsx
import React from "react";
import { Box, Typography } from "@mui/material";
import { useNavigate, useParams } from "react-router-dom";

import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { projectSlugMatches } from "../utils/projectSlug";

// Banner
import HeroBanner from "./HeroBanner";

// Images used in slides
import deepWorkImg from "./undraw_deep-work_muov.svg";
import typingCodeImg from "./undraw_typing-code_6t2b.svg";
import PredictiveAnalytics from "./undraw_predictive-analytics_6vi1.svg";

// Cards section (the new overview grid)
import ProjectFeatureCards from "./overview/ProjectFeatureCards";

// Providers required by cards (for counts, etc.)
import { ApiProvider } from "../context/ApiContext";
import { GatewayProvider } from "../context/GatewayContext";

// Wizard header to show after clicking "Get Start"
import StepHeader from "./overview/StepHeader";

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

  // Toggle wizard vs. banner+cards
  const [showWizard, setShowWizard] = React.useState(false);

  const orgHandle = params.orgHandle ?? organization?.handle ?? "";
  const projectSlug = params.projectHandle ?? "";

  const resolvedProject = React.useMemo(() => {
    if (selectedProject) return selectedProject;
    if (!projectSlug) return null;
    return (
      projects.find((p) => projectSlugMatches(p.name, p.id, projectSlug)) ??
      null
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

  return (
    <>
      {/* Header always visible */}
      <Box textAlign="center" mt={3}>
        <Typography variant="h4" fontWeight={700}>
          Welcome to {displayName}
        </Typography>
        <Typography color="text.secondary">
          Manage all aspects of the {displayName} project from here.
        </Typography>
      </Box>

      {showWizard ? (
        // Only the wizard when Get Start is clicked
        <Box
          maxHeight={"80vh"}
          position="relative"
          maxWidth={1200}
          mx="auto"
          mt={4}
          px={6}
        >
          <StepHeader />
        </Box>
      ) : (
        // Otherwise show Banner + Cards
        <>
          {/* Banner */}
          <Box mt={3} pr={6} pl={6}>
            <HeroBanner
              onStart={() => setShowWizard(true)}
              slides={[
                {
                  id: "1",
                  title: "Create a Gateway in minutes",
                  subtitle:
                    "Spin up a Hybrid or Cloud gateway and start proxying traffic with a single command.",
                  imageUrl: typingCodeImg,
                },
                {
                  id: "2",
                  title: "Import and discover your APIs",
                  subtitle:
                    "Push your OpenAPI / AsyncAPI definition and curate them with tags, versions, and contexts.",
                  imageUrl: deepWorkImg,
                },
                {
                  id: "3",
                  title: "Validate & monitor in one place",
                  subtitle:
                    "Run smoke tests and observe latency, errors, and throughput—before and after deploy.",
                  imageUrl: PredictiveAnalytics,
                },
              ]}
            />
          </Box>

          {/* Cards */}
          <Box mt={1.5} pr={6} pl={6}>
            <ApiProvider>
              <GatewayProvider>
                <ProjectFeatureCards
                  orgHandle={orgHandle}
                  projectSlug={projectSlug}
                />
              </GatewayProvider>
            </ApiProvider>
          </Box>
        </>
      )}
    </>
  );
};

export default ProjectOverview;
