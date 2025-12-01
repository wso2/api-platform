// src/App.tsx
import React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import MainLayout from "./layout/MainLayout";

import { useOrganization } from "./context/OrganizationContext";
import { useProjects } from "./context/ProjectContext";
import { projectSlugFromName } from "./utils/projectSlug";

import ScenarioLanding from "./components/ScenarioLanding";
import AppRoutes from "./routes";
import ExposeServiceWizard from "./pages/userScenarios/ExposeServiceWizard";
import PublishPortalWizard from "./pages/userScenarios/PublishPortalWizard";
import DesignApiWizard from "./pages/userScenarios/DesignApiWizard";

type ExperienceStage = "landing" | "wizard" | "platform";
type WizardType = "expose-service" | "publish-portal" | "design-api" | null;
const EXPERIENCE_STAGE_KEY = "apim-platform-experience-stage";

const App: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();

  const [experienceStage, setExperienceStage] = React.useState<ExperienceStage>(
    () => {
      if (typeof window === "undefined") return "landing";
      const stored = window.localStorage.getItem(EXPERIENCE_STAGE_KEY);
      return stored === "platform" ? "platform" : "landing";
    }
  );

  const [activeWizardType, setActiveWizardType] = React.useState<WizardType>(null);

  const { organization } = useOrganization();
  const { selectedProject } = useProjects();

  const defaultOrgPath = React.useMemo(() => {
    if (!organization) return "/";
    const projectSegment = selectedProject
      ? `/${projectSlugFromName(selectedProject.name, selectedProject.id)}`
      : "";
    return `/${organization.handle}${projectSegment}/overview`;
  }, [organization, selectedProject]);

  React.useEffect(() => {
    if (experienceStage === "platform" && typeof window !== "undefined") {
      window.localStorage.setItem(EXPERIENCE_STAGE_KEY, "platform");
    }
  }, [experienceStage]);

  const isRootScenarioPath = location.pathname === "/";
  const isUserScenarioPath = location.pathname === "/userSenario";

  const handleScenarioSkip = React.useCallback(() => {
    setExperienceStage("platform");
    // Avoid navigating to "/" before org/project is ready
    if (defaultOrgPath !== "/") {
      navigate(defaultOrgPath, { replace: true });
    }
  }, [defaultOrgPath, navigate]);

  const handleScenarioContinue = React.useCallback(
    (scenarioId: string) => {
      if (
        scenarioId === "expose-service" ||
        scenarioId === "publish-portal" ||
        scenarioId === "design-api"
      ) {
        setExperienceStage("wizard");
        setActiveWizardType(scenarioId as WizardType);
      } else {
        setExperienceStage("platform");
        setActiveWizardType(null);
      }
      if (defaultOrgPath !== "/") {
        navigate(defaultOrgPath, { replace: true });
      }
    },
    [defaultOrgPath, navigate]
  );

  const handleWizardFinish = React.useCallback(() => {
    setExperienceStage("platform");
    setActiveWizardType(null);
    if (defaultOrgPath !== "/") {
      navigate(defaultOrgPath, { replace: true });
    }
  }, [defaultOrgPath, navigate]);

  const handleBackToChoices = React.useCallback(() => {
    setExperienceStage("landing");
    setActiveWizardType(null);
    navigate("/userSenario", { replace: true });
  }, [navigate]);

  let layoutContent: React.ReactNode = null;
  if (experienceStage === "wizard") {
    if (activeWizardType === "publish-portal") {
      layoutContent = (
        <PublishPortalWizard
          onBackToChoices={handleBackToChoices}
          onSkip={handleScenarioSkip}
          onFinish={handleWizardFinish}
        />
      );
    } else if (activeWizardType === "design-api") {
      layoutContent = (
        <DesignApiWizard
          onBackToChoices={handleBackToChoices}
          onSkip={handleScenarioSkip}
          onFinish={handleWizardFinish}
        />
      );
    } else {
      layoutContent = (
        <ExposeServiceWizard
          onBackToChoices={handleBackToChoices}
          onSkip={handleScenarioSkip}
          onFinish={handleWizardFinish}
        />
      );
    }
  } else {
    layoutContent = <AppRoutes />;
  }

    if (isRootScenarioPath || isUserScenarioPath) {
    return (
      <MainLayout>
        <ScenarioLanding
          onContinue={handleScenarioContinue}
          onSkip={handleScenarioSkip}
        />
      </MainLayout>
    );
  }

  return (
    <MainLayout>{layoutContent}</MainLayout>
  );
};

export default App;
