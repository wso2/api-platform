// src/components/LeftSidebar.tsx
import React from "react";
import {
  Drawer,
  Toolbar,
  List,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Collapse,
  IconButton,
  Box,
} from "@mui/material";
import { alpha } from "@mui/material/styles";
import {
  Link as RouterLink,
  useLocation,
  useMatch,
  useResolvedPath,
} from "react-router-dom";

import HubOutlinedIcon from "@mui/icons-material/HubOutlined";
import DnsOutlinedIcon from "@mui/icons-material/DnsOutlined";
import SettingsIcon from "@mui/icons-material/Settings";
import ExpandLess from "@mui/icons-material/ExpandLess";
import ExpandMore from "@mui/icons-material/ExpandMore";

import MenuOverview from "./src/Icons/generated/MenuOverview";
import MenuProjectSheet from "./src/Icons/generated/MenuProjectSheet";
import MenuPolicies from "./src/Icons/generated/MenuPolicies";
import MenuAPIDefinition from "./src/Icons/generated/MenuAPIDefinition";
import MenuDevelop from "./src/Icons/generated/MenuDevelop";
import MenuTest from "./src/Icons/generated/MenuTest";
import MenuDeploy from "./src/Icons/generated/MenuDeploy";
import MenuBusinessInsights from "./src/Icons/generated/MenuBusinessInsights";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { isRootLevelSegment, splitPathSegments } from "../utils/navigation";
import { projectSlugFromName, projectSlugMatches } from "../utils/projectSlug";
import { useApisContext } from "../context/ApiContext";

export const LEFT_DRAWER_WIDTH = 240;

const escapeForRegex = (value: string) =>
  value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

function useActive(to: string, end = false) {
  const resolved = useResolvedPath(to);
  const match = useMatch({ path: resolved.pathname, end });
  return Boolean(match);
}

const LeftSidebar: React.FC = () => {
  const location = useLocation();

  const segments = React.useMemo(
    () => splitPathSegments(location.pathname),
    [location.pathname]
  );
  const apisChildActive = segments.includes("apis");
  const apiOverviewActive = segments.includes("apioverview");
  const gatewaysChildActive = segments.includes("gateways");
  const mcpChildActive = segments.includes("mcp");
  const prodChildActive = segments.includes("products");
  const portalsChildActive = segments.includes("portals");
  const policiesChildActive = segments.includes("policies");
  const adminChildActive = segments.includes("admin");
  const { organization, organizations } = useOrganization();
  const { selectedProject, projects } = useProjects();
  const { currentApiSlug: activeApiSlug } = useApisContext();

  const firstSegment = segments[0] ?? null;
  const isRootLevel = firstSegment && isRootLevelSegment(firstSegment);
  const candidateOrgFromSegments =
    firstSegment &&
    (!isRootLevel ||
      segments.length > 1 ||
      organizations.some((org) => org.handle === firstSegment))
      ? firstSegment
      : null;

  const orgHandle = organization?.handle ?? candidateOrgFromSegments ?? "";
  const baseOrgPath = orgHandle ? `/${orgHandle}` : "";

  const orgIndex = orgHandle ? segments.indexOf(orgHandle) : -1;
  const restSegments =
    orgIndex !== -1 ? segments.slice(orgIndex + 1) : segments;

  let projectSlugFromPath: string | null = null;
  if (restSegments.length > 1) {
    const candidate = restSegments[0];
    if (candidate && !isRootLevelSegment(candidate)) {
      const match = projects.find((project) =>
        projectSlugMatches(project.name, project.id, candidate)
      );
      projectSlugFromPath = match
        ? projectSlugFromName(match.name, match.id)
        : candidate;
    }
  } else if (restSegments.length === 1) {
    const candidate = restSegments[0];
    if (candidate && !isRootLevelSegment(candidate)) {
      const match = projects.find((project) =>
        projectSlugMatches(project.name, project.id, candidate)
      );
      if (match) {
        projectSlugFromPath = projectSlugFromName(match.name, match.id);
      }
    }
  }

  const selectedProjectSlug = selectedProject
    ? projectSlugFromName(selectedProject.name, selectedProject.id)
    : null;
  const effectiveProjectSlug = selectedProjectSlug ?? projectSlugFromPath;
  const projectBasePath = effectiveProjectSlug
    ? `${baseOrgPath}/${effectiveProjectSlug}`
    : null;

  const apiOverviewIndex = segments.findIndex(
    (segment) => segment.toLowerCase() === "apioverview"
  );
  const slugFromPath =
    apiOverviewIndex > 0 ? segments[apiOverviewIndex - 1] : null;
  const effectiveApiSlug = slugFromPath ?? activeApiSlug ?? null;
  const apiOverviewPath = React.useMemo(() => {
    if (!effectiveApiSlug) {
      return projectBasePath
        ? `${projectBasePath}/apis`
        : `${baseOrgPath}/apis`;
    }
    return projectBasePath
      ? `${projectBasePath}/${effectiveApiSlug}/apioverview`
      : `${baseOrgPath}/${effectiveApiSlug}/apioverview`;
  }, [effectiveApiSlug, projectBasePath, baseOrgPath]);

  const overviewPath = React.useMemo(() => {
    if (!orgHandle) {
      return "/overview";
    }
    return projectBasePath
      ? `${projectBasePath}/overview`
      : `${baseOrgPath}/overview`;
  }, [orgHandle, projectBasePath, baseOrgPath]);

  const overviewSelected = React.useMemo(() => {
    if (!orgHandle) {
      return location.pathname === "/overview";
    }
    const escapedOrg = escapeForRegex(orgHandle);
    const slugPart = effectiveProjectSlug
      ? `(?:/${escapeForRegex(effectiveProjectSlug)})?`
      : "";
    const regex = new RegExp(`^/${escapedOrg}${slugPart}/overview$`, "i");
    return regex.test(location.pathname);
  }, [orgHandle, effectiveProjectSlug, location.pathname]);

  const hasSelectedApi = Boolean(effectiveApiSlug);

  const [openApis, setOpenApis] = React.useState(false);
  const [openMcp, setOpenMcp] = React.useState(false);
  const [openProducts, setOpenProducts] = React.useState(false);

  React.useEffect(
    () => setOpenApis(hasSelectedApi && (apisChildActive || apiOverviewActive)),
    [apisChildActive, apiOverviewActive, hasSelectedApi]
  );
  React.useEffect(() => setOpenMcp(mcpChildActive), [mcpChildActive]);
  React.useEffect(() => setOpenProducts(prodChildActive), [prodChildActive]);

  React.useEffect(() => {
    const handler = (e: Event) => {
      const detail = (e as CustomEvent).detail as
        | { group?: string }
        | undefined;
      switch (detail?.group) {
        case "apis":
          setOpenApis(true);
          break;
        case "mcp":
          setOpenMcp(true);
          break;
        case "products":
          setOpenProducts(true);
          break;
      }
    };
    window.addEventListener("open-submenu", handler as EventListener);
    return () =>
      window.removeEventListener("open-submenu", handler as EventListener);
  }, []);

  // ===== LOOK & FEEL =====
  const railBg = "#24252fff"; // deep navy rail
  const idle = "#C7D7F2"; // light blue (unselected icon/text)
  const idleDim = alpha("#C7D7F2", 0.8);
  const hoverBg = alpha("#FFFFFF", 0.06);

  // Selected top-level pill (card)
  const cardBg = alpha("#FFFFFF", 0.08);
  const cardBorder = "rgba(255,255,255,0.10)";
  const cardBorderHover = "rgba(255,255,255,0.16)";
  const mint = "#11dd9d";

  // Base style for top-level items (unselected = transparent)
  const itemSx = {
    mx: 1,
    mb: 1.25,
    px: 1.25,
    minHeight: 48,
    borderRadius: 2,
    backgroundColor: "transparent",
    border: "1px solid transparent",
    color: idle,
    transition:
      "background-color 120ms ease, border-color 120ms ease, color 120ms ease",
    "& .MuiListItemIcon-root": {
      minWidth: 36,
      color: idle,
    },
    "&:hover": { backgroundColor: hoverBg },
    "&.Mui-selected": {
      backgroundColor: cardBg,
      border: `1px solid ${cardBorder}`,
      color: mint,
      "& .MuiListItemIcon-root": { color: mint },
      "&:hover": { borderColor: cardBorderHover, backgroundColor: cardBg },
      boxShadow: "inset 0 0 0 1px rgba(255,255,255,0.06)",
    },
    "&.Mui-focusVisible": {
      outline: `2px solid ${alpha(mint, 0.45)}`,
      outlineOffset: 2,
    },
  } as const;

  // Child items (submenu). Transparent always; ONLY selected shows mint bar.
  const childItemSx = {
    mx: 1.25,
    mb: 0.5,
    px: 1.25,
    minHeight: 44,
    borderRadius: 1.5,
    position: "relative",
    backgroundColor: "transparent",
    border: "1px solid transparent",
    color: idleDim,
    "& .MuiListItemIcon-root": { color: idleDim, minWidth: 36 },
    "&:hover": { backgroundColor: alpha("#FFFFFF", 0.04) },
    "&.Mui-selected": {
      color: mint,
      "& .MuiListItemIcon-root": { color: mint },
      backgroundColor: "transparent",
      // tiny mint bar at the far left ONLY when selected
      "&::before": {
        content: '""',
        position: "absolute",
        left: 6, // snug to rail side
        top: 8,
        bottom: 8,
        width: 3,
        borderRadius: 2,
        backgroundColor: mint,
      },
    },
  } as const;

  const NavItem = ({
    to,
    end,
    icon,
    label,
    sx,
    child = false,
    selectedOverride,
    disabled,
  }: {
    to: string;
    end?: boolean;
    icon: React.ReactNode;
    label: string;
    sx?: any;
    child?: boolean;
    selectedOverride?: boolean;
    disabled?: boolean;
  }) => {
    const active =
      !disabled && (selectedOverride ?? useActive(to, Boolean(end)));
    return (
      <ListItemButton
        component={(disabled ? "div" : RouterLink) as any}
        to={disabled ? undefined : to}
        disabled={disabled}
        selected={active}
        sx={{
          ...(child ? childItemSx : itemSx),
          ...sx,
          opacity: disabled ? 0.5 : 1,
          cursor: disabled ? "not-allowed" : "pointer",
        }}
      >
        <ListItemIcon sx={{ minWidth: 34 }}>
          <Box
            sx={{
              width: 28,
              height: 28,
              borderRadius: 1.2,
              display: "grid",
              placeItems: "center",
              color: "inherit",
            }}
          >
            {icon}
          </Box>
        </ListItemIcon>
        <ListItemText
          primary={label}
          primaryTypographyProps={{ fontSize: 14 }}
        />
      </ListItemButton>
    );
  };

  const ParentRow: React.FC<{
    to: string;
    label: string;
    icon: React.ReactNode;
    open: boolean;
    setOpen: (v: boolean) => void;
    selected: boolean;
  }> = ({ to, label, icon, open, setOpen, selected }) => (
    <ListItemButton
      component={RouterLink}
      to={to}
      selected={selected}
      sx={itemSx}
    >
      <ListItemIcon sx={{ minWidth: 34 }}>
        <Box
          sx={{
            width: 28,
            height: 28,
            borderRadius: 1.2,
            display: "grid",
            placeItems: "center",
            color: "inherit",
          }}
        >
          {icon}
        </Box>
      </ListItemIcon>
      <ListItemText primary={label} primaryTypographyProps={{ fontSize: 14 }} />
      <IconButton
        size="small"
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setOpen(!open);
        }}
        sx={{
          color: "inherit",
          ml: 0.5,
          "&:hover": { backgroundColor: alpha("#FFFFFF", 0.06) },
        }}
      >
        {open ? (
          <ExpandLess fontSize="small" />
        ) : (
          <ExpandMore fontSize="small" />
        )}
      </IconButton>
    </ListItemButton>
  );

  const apisSelected = apisChildActive || apiOverviewActive;
  const gatewaysSelected = gatewaysChildActive;
  const mcpSelected = mcpChildActive;
  const productsSelected = prodChildActive;
  const portalsSelected = portalsChildActive;
  const policiesSelected = policiesChildActive;
  const adminSelected = adminChildActive;

  return (
    <Drawer
      variant="permanent"
      sx={{
        width: LEFT_DRAWER_WIDTH,
        flexShrink: 0,
        "& .MuiDrawer-paper": {
          width: LEFT_DRAWER_WIDTH,
          boxSizing: "border-box",
          bgcolor: railBg,
          color: "#fff",
          borderRight: "none",
          paddingTop: 0.5,
        },
      }}
    >
      <Toolbar />
      <List sx={{ px: 0.25, mt: 0.5 }}>
        {/* Top-level items */}
        <NavItem
          to={overviewPath}
          icon={<MenuOverview fontSize="small" />}
          label="Overview"
          selectedOverride={overviewSelected}
        />
        <NavItem
          to={
            projectBasePath
              ? `${projectBasePath}/gateways`
              : `${baseOrgPath}/gateways`
          }
          end
          icon={<HubOutlinedIcon fontSize="small" />}
          label="Gateways"
          selectedOverride={gatewaysSelected}
        />
        <NavItem
          to={
            projectBasePath
              ? `${projectBasePath}/portals`
              : `${baseOrgPath}/portals`
          }
          end
          icon={<MenuProjectSheet fontSize="small" />}
          label="Portals"
          selectedOverride={portalsSelected}
        />
        <NavItem
          to={
            projectBasePath
              ? `${projectBasePath}/policies`
              : `${baseOrgPath}/policies`
          }
          end
          icon={<MenuPolicies fontSize="small" />}
          label="Policies"
          selectedOverride={policiesSelected}
        />

        {/* API Proxies group */}
        <ParentRow
          to={
            projectBasePath ? `${projectBasePath}/apis` : `${baseOrgPath}/apis`
          }
          label="APIs"
          icon={<MenuAPIDefinition fontSize="small" />}
          open={hasSelectedApi && openApis}
          setOpen={setOpenApis}
          selected={apisSelected}
        />
        <Collapse in={hasSelectedApi && openApis} timeout="auto" unmountOnExit>
          <NavItem
            child
            to={apiOverviewPath}
            icon={<MenuOverview fontSize="small" />}
            label="Overview"
            selectedOverride={apiOverviewActive}
          />
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/apis/develop`
                : `${baseOrgPath}/apis/develop`
            }
            icon={<MenuDevelop fontSize="small" />}
            label="Deploy"
          />
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/apis/test`
                : `${baseOrgPath}/apis/test`
            }
            icon={<MenuTest fontSize="small" />}
            label="Test"
          />
        </Collapse>

        {/* MCP Servers group */}
        <ParentRow
          to={projectBasePath ? `${projectBasePath}/mcp` : `${baseOrgPath}/mcp`}
          label="MCP"
          icon={<DnsOutlinedIcon fontSize="small" />}
          open={openMcp}
          setOpen={setOpenMcp}
          selected={mcpSelected}
        />
        <Collapse in={openMcp} timeout="auto" unmountOnExit>
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/mcp/test`
                : `${baseOrgPath}/mcp/test`
            }
            icon={<MenuDevelop fontSize="small" />}
            label="Gateways"
          />
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/mcp/deploy`
                : `${baseOrgPath}/mcp/deploy`
            }
            icon={<MenuDeploy fontSize="small" />}
            label="Consumers"
          />
        </Collapse>

        {/* API Products group */}
        <ParentRow
          to={
            projectBasePath
              ? `${projectBasePath}/products`
              : `${baseOrgPath}/products`
          }
          label="API Products"
          icon={<MenuBusinessInsights fontSize="small" />}
          open={openProducts}
          setOpen={setOpenProducts}
          selected={productsSelected}
        />
        <Collapse in={openProducts} timeout="auto" unmountOnExit>
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/products/test`
                : `${baseOrgPath}/products/test`
            }
            icon={<MenuDevelop fontSize="small" />}
            label="Gateways"
          />
          <NavItem
            child
            to={
              projectBasePath
                ? `${projectBasePath}/products/deploy`
                : `${baseOrgPath}/products/deploy`
            }
            icon={<MenuTest fontSize="small" />}
            label="Consumers"
          />
        </Collapse>

        <NavItem
          to={
            projectBasePath
              ? `${projectBasePath}/admin`
              : `${baseOrgPath}/admin`
          }
          end
          icon={<SettingsIcon fontSize="small" />}
          label="Admin"
          selectedOverride={adminSelected}
        />
      </List>
    </Drawer>
  );
};

export default LeftSidebar;
