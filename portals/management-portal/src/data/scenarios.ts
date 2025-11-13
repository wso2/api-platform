import ShieldOutlinedIcon from "@mui/icons-material/ShieldOutlined";
import AutoFixHighOutlinedIcon from "@mui/icons-material/AutoFixHighOutlined";
import TravelExploreOutlinedIcon from "@mui/icons-material/TravelExploreOutlined";
import DrawOutlinedIcon from "@mui/icons-material/DrawOutlined";
import HubOutlinedIcon from "@mui/icons-material/HubOutlined";
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
};

export const SCENARIOS: Scenario[] = [
  {
    id: "expose-service",
    title: "Expose My Service Securely",
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
    id: "design-api",
    title: "Design a New API",
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
    id: "consume-ai",
    title: "Securely Consume AI APIs",
    description: "Connect to OpenAI or Anthropic through a secure gateway.",
    icon: AutoFixHighOutlinedIcon,
    steps: [
      { title: "Select Provider", subtitle: "Pick OpenAI, Anthropic, etc." },
      { title: "Apply Policies", subtitle: "Set rate limits and auth rules." },
      { title: "Monitor Usage", subtitle: "Track cost and performance." },
    ],
    imageSrc: Comingsoon,
  },
  {
    id: "publish-portal",
    title: "Publish to Developer Portal",
    description: "Make your API visible for discovery and subscription.",
    icon: HubOutlinedIcon,
    steps: [
      { title: "Import Backend", subtitle: "Sync your OpenAPI definition." },
      { title: "Add Plans", subtitle: "Define pricing and access tiers." },
      { title: "Launch Portal", subtitle: "Announce and share your API." },
    ],
    imageSrc: Scenario3,
  },
  {
    id: "discover",
    title: "Discover APIs",
    description: "Browse and test APIs across your organization.",
    icon: TravelExploreOutlinedIcon,
    steps: [
      { title: "Search Catalog", subtitle: "Filter by team or tag." },
      { title: "Subscribe", subtitle: "Request access to products." },
      { title: "Try It", subtitle: "Use the built-in test console." },
    ],
    imageSrc: Comingsoon,
  },
];
