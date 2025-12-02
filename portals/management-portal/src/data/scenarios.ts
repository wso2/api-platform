import ShieldOutlinedIcon from "@mui/icons-material/ShieldOutlined";
import AutoFixHighOutlinedIcon from "@mui/icons-material/AutoFixHighOutlined";
import TravelExploreOutlinedIcon from "@mui/icons-material/TravelExploreOutlined";
import DrawOutlinedIcon from "@mui/icons-material/DrawOutlined";
import HubOutlinedIcon from "@mui/icons-material/HubOutlined";
import AnalyticsOutlinedIcon from "@mui/icons-material/AnalyticsOutlined";
import Scenario1 from "../images/Scenarios/Scenario1.svg";
import Scenario2 from "../images/Scenarios/Scenario2.svg";
import Scenario3 from "../images/Scenarios/Scenario3.svg";
import Comingsoon from "../images/Scenarios/Comingsoon.svg";

export type Scenario = {
  id: string;
  title: string;
  description: string;
  badge?: string;
  icon: typeof ShieldOutlinedIcon;
  steps: { title: string; subtitle: string }[];
  imageSrc?: string;
  comingSoon?: boolean;
};

export const SCENARIOS: Scenario[] = [
  {
    id: "expose-service",
    title: "Expose my service through an API Gateway",
    description:
      "Publish your first service safely through a managed API Gateway.",
    badge: "Recommended",
    icon: ShieldOutlinedIcon,
    steps: [
      {
        title: "Add Your First Gateway",
        subtitle: "Connect and secure your backend service.",
      },
      {
        title: "Design API",
        subtitle: "Define resources and methods for your service.",
      },
      {
        title: "Test Your API",
        subtitle: "Validate requests before publishing.",
      },
    ],
    imageSrc: Scenario1,
  },
  {
    id: "consume-ai",
    title: "Manage my AI APIs",
    description: "Connect to OpenAI or Anthropic through a secure gateway.",
    badge: "Coming Soon",
    icon: AutoFixHighOutlinedIcon,
    comingSoon: true,
    steps: [
      { title: "Select Provider", subtitle: "Pick OpenAI, Anthropic, etc." },
      { title: "Apply Policies", subtitle: "Set rate limits and auth rules." },
      { title: "Monitor Usage", subtitle: "Track cost and performance." },
    ],
    imageSrc: Comingsoon,
  },
  {
    id: "design-api",
    title: "Design an API",
    description:
      "Use the WSO2 API Designer in VS Code to build and document APIs fast.",
    icon: DrawOutlinedIcon,
    steps: [
      {
        title: "Install Extension",
        subtitle: "Add the WSO2 API Designer from VS Code Extensions.",
      },
      {
        title: "Open your OpenAPI specification",
        subtitle: "Launch the designer and model endpoints visually.",
      },
      {
        title: "Test & Document",
        subtitle: "Mock responses, run quick tests, and generate docs.",
      },
    ],
    imageSrc: Scenario2,
  },
  {
    id: "publish-portal",
    title: "Publish my APIs to a Developer Portal",
    description: "Make your API visible for discovery and subscription.",
    icon: HubOutlinedIcon,
    steps: [
      { title: "Create or Select API", subtitle: "Import from URL or choose an existing API." },
      { title: "Publish to Developer Portal", subtitle: "Select portal and configure visibility options." },
    ],
    imageSrc: Scenario3,
  },
  {
    id: "discover",
    title: "Discover and consume APIs",
    description: "Browse and test APIs across your organization.",
    badge: "Coming Soon",
    icon: TravelExploreOutlinedIcon,
    comingSoon: true,
    steps: [
      { title: "Search Catalog", subtitle: "Filter by team or tag." },
      { title: "Subscribe", subtitle: "Request access to products." },
      { title: "Try It", subtitle: "Use the built-in test console." },
    ],
    imageSrc: Comingsoon,
  },
  {
    id: "setup-analytics",
    title: "Setup Analytics for my APIs",
    description: "Monitor API performance, usage patterns, and business metrics.",
    badge: "Coming Soon",
    icon: AnalyticsOutlinedIcon,
    comingSoon: true,
    steps: [
      { title: "Configure Data Sources", subtitle: "Connect analytics backend." },
      { title: "Create Dashboards", subtitle: "Build custom analytics views." },
      { title: "Set Alerts", subtitle: "Monitor thresholds and get notified." },
    ],
    imageSrc: Comingsoon,
  },
];
