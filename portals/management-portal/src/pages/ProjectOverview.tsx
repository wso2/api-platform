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

import { GatewayProvider } from "../context/GatewayContext";
import GatewayWizard from "./overview/StepHeader";

// Wizard header to show after clicking "Get Start"

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
      <Box textAlign="center" mt={1}>
        <Typography variant="h3" fontWeight={600}>
          Welcome to {displayName} Project
        </Typography>
        <Typography color="#666666" mb={2}>
          Manage all aspects of the {displayName} project from here.
        </Typography>
      </Box>

      {showWizard ? (
        <Box maxWidth={1200} mx="auto" mt={4} px={6}>
          <GatewayWizard onFinish={() => setShowWizard(false)} />
        </Box>
      ) : (
        <>
          {/* Banner */}
          <Box mt={3} pr={6} pl={6}>
            <HeroBanner
              intervalMs={5000}
              slides={[
                {
                  id: "s1",
                  tag: "Quick Start",
                  title: "Create a Gateway in minutes",
                  subtitle:
                    "Spin up a Hybrid or Cloud gateway and start proxying traffic with a single command.",
                  ctaLabel: "Create Gateway",
                  imageNode: (
                    <Box
                      sx={{
                        width: 180,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      <Box
                        component="img"
                        src={typingCodeImg}
                        alt="Add API illustration"
                        sx={{ width: "100%", height: "auto" }}
                      />
                    </Box>
                  ),
                },
                {
                  id: "s2",
                  tag: "APIs",
                  title: "Import and discover your APIs",
                  subtitle:
                    "Push your OpenAPI / AsyncAPI definition and curate them with tags, versions, and contexts.",
                  ctaLabel: "Add APIs",
                  imageNode: (
                    <Box
                      sx={{
                        width: 180,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      <Box
                        component="img"
                        src={deepWorkImg}
                        alt="Add API illustration"
                        sx={{ width: "100%", height: "auto" }}
                      />
                    </Box>
                  ),
                },
                {
                  id: "s3",
                  tag: "Testing",
                  title: "Validate & monitor in one place",
                  subtitle:
                    "Run smoke tests and observe latency, errors, and throughput—before and after deploy.",
                  ctaLabel: "Run Tests",
                  imageNode: (
                    <Box
                      sx={{
                        width: 180,
                        borderRadius: 2,
                        display: "flex",
                        alignItems: "center",
                        justifyContent: "center",
                      }}
                    >
                      <Box
                        component="img"
                        src={PredictiveAnalytics}
                        alt="Predictive analytics illustration"
                        sx={{ width: "100%", height: "auto" }}
                      />
                    </Box>
                  ),
                },
              ]}
            />
          </Box>

          {/* Cards */}
          <Box mt={1.5} pr={6} pl={6}>
            <GatewayProvider>
              <ProjectFeatureCards
                orgHandle={orgHandle}
                projectSlug={projectSlug}
              />
            </GatewayProvider>
          </Box>
        </>
      )}
    </>
  );
};

export default ProjectOverview;
