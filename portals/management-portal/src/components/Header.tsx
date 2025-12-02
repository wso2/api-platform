// src/components/Header.tsx
import React from "react";
import {
  AppBar,
  Toolbar,
  Typography,
  Box,
  Avatar,
  IconButton,
  ButtonBase,
  Popover,
  Paper,
  TextField,
  InputAdornment,
  List,
  ListItemButton,
  ListItemText,
  Divider,
  Chip,
} from "@mui/material";
import KeyboardArrowDownRoundedIcon from "@mui/icons-material/KeyboardArrowDownRounded";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";
import AddRoundedIcon from "@mui/icons-material/AddRounded";
import SearchRoundedIcon from "@mui/icons-material/SearchRounded";
import CloseRoundedIcon from "@mui/icons-material/CloseRounded";
import { useNavigate, useParams, useLocation } from "react-router-dom";
import { useOrganization } from "../context/OrganizationContext";
import { useProjects } from "../context/ProjectContext";
import { projectSlugFromName, projectSlugMatches } from "../utils/projectSlug";
import {
  normalizeSegmentsForOrganization,
  normalizeSegmentsForProject,
} from "../utils/navigation";
import type { Project } from "../hooks/projects";
import { TextInput } from "./src";
import { useApisContext } from "../context/ApiContext";
import { slugify } from "../utils/slug";

/** Simple tag pill like MCP / Proxy on the right */
function TypeChip({ type }: { type?: string }) {
  if (!type) return null;
  return (
    <Chip
      label={type}
      size="small"
      sx={{
        ml: 1,
        bgcolor: "grey.100",
        color: "text.primary",
        borderRadius: 1,
        height: 24,
        "& .MuiChip-label": { px: 1 },
      }}
    />
  );
}

const Header: React.FC = () => {
  const navigate = useNavigate();
  const location = useLocation();
  const params = useParams<{ orgHandle?: string; projectHandle?: string }>();
  const { organization, organizations, setSelectedOrganization } =
    useOrganization();
  const { projects, selectedProject, setSelectedProject } = useProjects();
  const {
    apis,
    loading: apisLoading,
    refreshApis,
    selectApi,
    currentApi,
  } = useApisContext();

  const proxyPlaceholder = "Select API";
  const [proxy, setProxy] = React.useState(proxyPlaceholder);
  const [showOrg, setShowOrg] = React.useState(true);
  const [showProxy, setShowProxy] = React.useState(false);
  const [menuAnchor, setMenuAnchor] = React.useState<HTMLElement | null>(null);
  const [activeMenu, setActiveMenu] = React.useState<
    "project" | "proxy" | null
  >(null);

  const currentOrgHandle =
    organization?.handle ?? params.orgHandle ?? organizations[0]?.handle ?? "";
  const currentProjectSlugParam = params.projectHandle ?? null;
  const organizationName =
    organization?.name ?? organizations[0]?.name ?? "Select Organization";

  React.useEffect(() => {
    const slug = params.projectHandle;

    if (!slug) {
      return;
    }

    const match = projects.find((project) =>
      projectSlugMatches(project.name, project.id, slug)
    );

    if (match && (!selectedProject || selectedProject.id !== match.id)) {
      setSelectedProject(match);
    }
  }, [params.projectHandle, projects, selectedProject, setSelectedProject]);

  React.useEffect(() => {
    if (selectedProject) {
      return;
    }

    if (showProxy) {
      setShowProxy(false);
    }

    if (proxy !== proxyPlaceholder) {
      setProxy(proxyPlaceholder);
    }
  }, [selectedProject, showProxy, proxy, proxyPlaceholder]);

  React.useEffect(() => {
    if (currentApi && selectedProject) {
      if (proxy !== currentApi.name) {
        setProxy(currentApi.name);
      }
      if (!showProxy) {
        setShowProxy(true);
      }
    } else if (!currentApi && selectedProject && proxy !== proxyPlaceholder) {
      setProxy(proxyPlaceholder);
    }
  }, [
    currentApi,
    proxy,
    proxyPlaceholder,
    selectedProject,
    showProxy,
  ]);

  const projectMenuItems = React.useMemo<MenuItem[]>(
    () =>
      projects.map((project) => ({
        label: project.name,
      })),
    [projects]
  );

  const organizationMenuItems = React.useMemo<MenuItem[]>(
    () =>
      organizations.map((org) => ({
        label: org.name,
      })),
    [organizations]
  );

  const apiMenuItems = React.useMemo<MenuItem[]>(() => {
    if (!selectedProject) return [];
    return apis
      .map<MenuItem>((api) => ({
        label: api.name,
        type: api.type ?? api.transport?.[0],
      }))
      .sort((a, b) => a.label.localeCompare(b.label));
  }, [apis, selectedProject]);

  const proxyMenuSections = React.useMemo<MenuSection[] | undefined>(() => {
    if (!apiMenuItems.length) return undefined;
    return [
      {
        title: "All APIs",
        items: apiMenuItems,
      },
    ];
  }, [apiMenuItems]);

  const hasApis = apiMenuItems.length > 0;

  const proxyValue = React.useMemo(
    () => (apisLoading && !hasApis ? "Loading APIs..." : proxy),
    [apisLoading, hasApis, proxy]
  );

  const closeActiveMenu = React.useCallback(() => {
    setActiveMenu(null);
    setMenuAnchor(null);
  }, []);

  const lastSelectedProjectNameRef = React.useRef<string | null>(null);
  React.useEffect(() => {
    if (selectedProject?.name) {
      lastSelectedProjectNameRef.current = selectedProject.name;
    }
  }, [selectedProject?.id, selectedProject?.name]);

  const currentProjectSlug = React.useMemo(() => {
    if (selectedProject) {
      return projectSlugFromName(selectedProject.name, selectedProject.id);
    }
    return currentProjectSlugParam;
  }, [selectedProject, currentProjectSlugParam]);

  const showProjectPicker = Boolean(
    selectedProject || lastSelectedProjectNameRef.current
  );
  const projectPickerValue =
    selectedProject?.name ??
    lastSelectedProjectNameRef.current ??
    "Select Project";

  const getRestSegments = React.useCallback(() => {
    if (!currentOrgHandle) {
      return [];
    }
    const segments = location.pathname.split("/").filter(Boolean);
    const orgIndex = segments.indexOf(currentOrgHandle);
    if (orgIndex === -1) {
      return [];
    }
    return segments.slice(orgIndex + 1);
  }, [location.pathname, currentOrgHandle]);

  const buildPathForProject = React.useCallback(
    (newSlug: string) => {
      const rest = getRestSegments();
      let restAfter = [...rest];
      if (currentProjectSlug && restAfter[0] === currentProjectSlug) {
        restAfter = restAfter.slice(1);
      }

      const normalizedRest = normalizeSegmentsForProject(restAfter);
      const restPath = normalizedRest.join("/");

      if (!currentOrgHandle) {
        return `/${newSlug}/${restPath}`;
      }

      return `/${currentOrgHandle}/${newSlug}/${restPath}`;
    },
    [currentOrgHandle, currentProjectSlug, getRestSegments]
  );

  const buildPathWithoutProject = React.useCallback(() => {
    const rest = getRestSegments();
    let restAfter = [...rest];
    if (currentProjectSlug && restAfter[0] === currentProjectSlug) {
      restAfter = restAfter.slice(1);
    }

    const normalizedRest = normalizeSegmentsForOrganization(restAfter);
    const restPath = normalizedRest.join("/");

    if (!currentOrgHandle) {
      return `/${restPath}`;
    }

    return `/${currentOrgHandle}/${restPath}`;
  }, [currentOrgHandle, currentProjectSlug, getRestSegments]);

  const buildApiOverviewPath = React.useCallback(
    (apiSlug: string, apiId: string) => {
      if (!apiSlug) {
        if (currentOrgHandle && currentProjectSlug) {
          return `/${currentOrgHandle}/${currentProjectSlug}/apis`;
        }
        if (currentOrgHandle) {
          return `/${currentOrgHandle}/apis`;
        }
        return "/apis";
      }
      const query = new URLSearchParams({ apiId }).toString();
      if (currentOrgHandle && currentProjectSlug) {
        return `/${currentOrgHandle}/${currentProjectSlug}/${apiSlug}/apioverview?${query}`;
      }
      if (currentOrgHandle) {
        return `/${currentOrgHandle}/${apiSlug}/apioverview?${query}`;
      }
      return `/${apiSlug}/apioverview?${query}`;
    },
    [currentOrgHandle, currentProjectSlug]
  );

  const navigateToApiList = React.useCallback(() => {
    if (currentOrgHandle && currentProjectSlug) {
      navigate(`/${currentOrgHandle}/${currentProjectSlug}/apis`);
      return;
    }
    if (currentOrgHandle) {
      navigate(`/${currentOrgHandle}/apis`);
      return;
    }
    navigate("/apis");
  }, [currentOrgHandle, currentProjectSlug, navigate]);

  const handleRevealProject = (event: React.MouseEvent<HTMLButtonElement>) => {
    setMenuAnchor(event.currentTarget);
    setActiveMenu("project");
  };

  const handleRevealProxy = (event: React.MouseEvent<HTMLButtonElement>) => {
    if (!selectedProject) return;
    if (!hasApis && !apisLoading) {
      refreshApis(selectedProject.id).catch(() => {
        /* errors handled in context */
      });
    }
    setMenuAnchor(event.currentTarget);
    setActiveMenu("proxy");
  };

  const navigateToProject = React.useCallback(
    (project: Project) => {
      const slug = projectSlugFromName(project.name, project.id);
      setSelectedProject(project);
      setShowProxy(false);
      setProxy(proxyPlaceholder);
      selectApi(null);
      navigate(buildPathForProject(slug));
    },
    [buildPathForProject, navigate, proxyPlaceholder, selectApi, setSelectedProject]
  );

  const handleMenuSelect = (label: string) => {
    if (!activeMenu) return;
    if (activeMenu === "project") {
      const project = projects.find((item) => item.name === label);
      if (project) {
        navigateToProject(project);
      }
    } else if (activeMenu === "proxy") {
      setProxy(label);
      setShowProxy(true);
      const matchedApi = apis.find((item) => item.name === label);
      if (matchedApi) {
        const apiSlug = slugify(matchedApi.name);
        selectApi(matchedApi, { slug: apiSlug });
        navigate(buildApiOverviewPath(apiSlug, matchedApi.id));
      }
    }
    closeActiveMenu();
  };

  const handleCreateProject = React.useCallback(() => {
    console.log("Create New clicked");
  }, []);

  const handleCreateProxy = React.useCallback(() => {
    console.log("Create New proxy clicked");
  }, []);

  const handleProjectChange = (label: string) => {
    const project = projects.find((item) => item.name === label);
    if (project) {
      navigateToProject(project);
    }
  };

  const handleOrganizationChange = (label: string) => {
    const org = organizations.find((item) => item.name === label);
    if (org) {
      setSelectedOrganization(org);
      setSelectedProject(null);
      setShowProxy(false);
      setProxy(proxyPlaceholder);
      selectApi(null);
      navigate(`/${org.handle}/overview`);
    }
  };

  return (
    <AppBar
      elevation={0}
      position="fixed"
      sx={{
        zIndex: (t) => t.zIndex.drawer + 1,
        bgcolor: "background.paper",
        color: "text.primary",
        borderBottom: "1px solid #e8e8ee",
        py: 0.5,
      }}
    >
      <Toolbar variant="dense" sx={{ minHeight: 52 }}>
        <ButtonBase
          sx={{
            display: 'flex',
            alignItems: 'center',
            gap: 1.25,
            marginRight: 7,
            '&:hover .mp-title': { color: 'primary.main' },
          }}
          onClick={() => navigate('/userScenario')}
          aria-label="Navigate to user scenarios"
        >
          <Avatar
            variant="rounded"
            sx={{
              width: 36,
              height: 36,
              fontSize: 14,
              fontWeight: 700,
              color: '#fff',
              boxShadow: '0 6px 18px rgba(4,150,105,0.30)',
              background: 'linear-gradient(180deg, rgba(4,150,105,0.95) 0%, #049669 100%)',
            }}
          >
            MP
          </Avatar>
          <Typography
            className="mp-title"
            variant="subtitle2"
            sx={{ fontWeight: 800, letterSpacing: 0.2 }}
          >
            Management Portal
          </Typography>
        </ButtonBase>

        <Box ml={1.5} display="flex" alignItems="center" gap={0.75}>
          {showOrg && (
            <FieldPicker
              label="Organization"
              value={organizationName}
              onChange={handleOrganizationChange}
              width={200}
              height={50}
              menuItems={
                organizationMenuItems.length
                  ? organizationMenuItems
                  : [{ label: organizationName }]
              }
              menuTitle="All Organizations"
              onRemove={() => {
                closeActiveMenu();
                setShowOrg(false);
                navigate("/");
              }}
            />
          )}

          {showOrg && !showProjectPicker && (
            <IconButton
              size="small"
              onClick={handleRevealProject}
              sx={{
                width: 28,
                height: 28,
                borderRadius: 1.25,
                bgcolor: "background.paper",
                border: "1px solid",
                borderColor: "divider",
                "&:hover": { bgcolor: "action.hover" },
                mx: 0.25,
              }}
            >
              <ChevronRightRoundedIcon fontSize="small" />
            </IconButton>
          )}

          {showProjectPicker && (
            <FieldPicker
              label="Project"
              value={projectPickerValue}
              onChange={handleProjectChange}
              width={200}
              height={50}
              menuItems={projectMenuItems}
              menuTitle="All Projects"
              onCreateNew={handleCreateProject}
              onRemove={() => {
                lastSelectedProjectNameRef.current = null;
                setSelectedProject(null);
                setShowProxy(false);
                setProxy(proxyPlaceholder);
                navigate(buildPathWithoutProject());
              }}
            />
          )}

          {showProjectPicker && !showProxy && (
            <IconButton
              size="small"
              onClick={handleRevealProxy}
              sx={{
                width: 28,
                height: 28,
                borderRadius: 1.25,
                bgcolor: "background.paper",
                border: "1px solid",
                borderColor: "divider",
                "&:hover": { bgcolor: "action.hover" },
                mx: 0.25,
              }}
            >
              <ChevronRightRoundedIcon fontSize="small" />
            </IconButton>
          )}

          {showProxy && (
            <FieldPicker
              label="Proxies/MCP"
              value={proxyValue}
              onChange={(label) => {
                setProxy(label);
              }}
              width={220}
              height={50}
              menuTitle="All APIs"
              menuSections={proxyMenuSections}
              menuItems={!proxyMenuSections ? apiMenuItems : undefined}
              onCreateNew={handleCreateProxy}
              onRemove={() => {
                closeActiveMenu();
                setShowProxy(false);
                setProxy(proxyPlaceholder);
                selectApi(null);
                navigateToApiList();
              }}
            />
          )}
        </Box>

        <Box sx={{ flex: 1 }} />
        <Box ml={1}>
          <Avatar alt="User" sx={{ width: 35, height: 35 }} />
        </Box>
      </Toolbar>
      <FieldPickerMenuPopover
        anchorEl={menuAnchor}
        open={Boolean(activeMenu)}
        onClose={closeActiveMenu}
        onSelect={handleMenuSelect}
        currentValue={activeMenu === "proxy" ? proxy : projectPickerValue}
        menuTitle={activeMenu === "proxy" ? "All APIs" : "All Projects"}
        menuItems={
          activeMenu === "project"
            ? projectMenuItems
            : activeMenu === "proxy" && !proxyMenuSections
            ? apiMenuItems
            : undefined
        }
        menuSections={activeMenu === "proxy" ? proxyMenuSections : undefined}
        onCreateNew={
          activeMenu === "project"
            ? handleCreateProject
            : activeMenu === "proxy"
            ? handleCreateProxy
            : undefined
        }
      />
    </AppBar>
  );
};

export default Header;

/** Extra-compact card-like picker with screenshot-style popover menu */
type MenuItem = { label: string; type?: string };
type MenuSection = { title?: string; items: MenuItem[] };

function FieldPickerMenuPopover({
  anchorEl,
  open,
  onClose,
  onSelect,
  menuTitle = "All Projects",
  onCreateNew,
  menuItems,
  menuSections,
  currentValue,
}: {
  anchorEl: HTMLElement | null;
  open: boolean;
  onClose: () => void;
  onSelect: (label: string) => void;
  menuTitle?: string;
  onCreateNew?: () => void;
  menuItems?: MenuItem[];
  menuSections?: MenuSection[];
  currentValue?: string;
}) {
  const [query, setQuery] = React.useState("");

  const sections = React.useMemo(() => {
    const q = query.trim().toLowerCase();
    const matchesQuery = ({ label, type }: MenuItem) => {
      if (!q) return true;
      return [label, type || ""].some((s) => s.toLowerCase().includes(q));
    };

    if (menuSections?.length) {
      return menuSections
        .map(({ title, items }) => ({
          title,
          items: items.filter(matchesQuery),
        }))
        .filter((section) => section.items.length > 0);
    }

    if (menuItems?.length) {
      const items = menuItems.filter(matchesQuery);
      return [
        {
          title: menuTitle,
          items,
        },
      ];
    }

    return [];
  }, [menuItems, menuSections, menuTitle, query]);

  const hasResults = sections.some((section) => section.items.length > 0);

  const handleClose = React.useCallback(() => {
    setQuery("");
    onClose();
  }, [onClose]);

  const handleSelect = React.useCallback(
    (label: string) => {
      onSelect(label);
      setQuery("");
      onClose();
    },
    [onClose, onSelect]
  );

  return (
    <Popover
      open={open}
      anchorEl={anchorEl}
      onClose={handleClose}
      anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
      transformOrigin={{ vertical: "top", horizontal: "left" }}
      PaperProps={{
        elevation: 6,
        sx: {
          mt: 1,
          borderRadius: 2,
          width: 250,
          overflow: "hidden",
        },
      }}
    >
      <Paper sx={{ p: 2, boxShadow: "none" }}>
        <TextInput
          fullWidth
          placeholder="Search"
          value={query}
          onChange={(v) => setQuery(v)}
          size="small"
          InputProps={{
            sx: {
              borderRadius: 1.5,
              height: 40,
            },
            endAdornment: (
              <InputAdornment position="end">
                <SearchRoundedIcon fontSize="small" />
              </InputAdornment>
            ),
          }}
          testId={""}
        />

        {onCreateNew && (
          <List dense disablePadding sx={{ mt: 1 }}>
            <ListItemButton
              onClick={() => {
                onCreateNew();
                handleClose();
              }}
              sx={{
                borderRadius: 1,
                px: 1,
                height: 40,
                "& .MuiListItemText-primary": { fontWeight: 600 },
              }}
            >
              <AddRoundedIcon fontSize="small" style={{ marginRight: 8 }} />
              <ListItemText primary="Create New" />
            </ListItemButton>
          </List>
        )}

        <Divider sx={{ my: 1.5 }} />

        {hasResults ? (
          sections.map((section, index) => (
            <React.Fragment key={(section.title || "section") + index}>
              {index > 0 && <Divider sx={{ my: 1.5 }} />}

              {section.title && (
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ px: 1, mb: 0.75 }}
                >
                  {section.title}
                </Typography>
              )}

              <List dense disablePadding>
                {section.items.map((item) => {
                  const isSelected = item.label === currentValue;
                  return (
                    <ListItemButton
                      key={item.label + (item.type || "")}
                      selected={Boolean(currentValue && isSelected)}
                      onClick={() => handleSelect(item.label)}
                      sx={{
                        px: 1,
                        height: 44,
                        borderRadius: 1,
                        mb: 0.25,
                      }}
                    >
                      <ListItemText
                        primary={item.label}
                        primaryTypographyProps={{
                          sx: { fontSize: 14, fontWeight: 500 },
                        }}
                      />
                      <TypeChip type={item.type} />
                    </ListItemButton>
                  );
                })}
              </List>
            </React.Fragment>
          ))
        ) : (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ px: 1, py: 1 }}
          >
            No results
          </Typography>
        )}
      </Paper>
    </Popover>
  );
}

function FieldPicker({
  label,
  value,
  onChange,
  width = 210,
  height = 48,
  menuTitle = "All Projects",
  onCreateNew,
  // When provided, we render sections (e.g. “Recent”, “All …”)
  menuSections,
  // Fallback to a single section when only menuItems are given
  menuItems,
  onRemove,
  autoOpen,
  onAutoOpenHandled,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  width?: number;
  height?: number;
  /** Subheader shown over the list */
  menuTitle?: string;
  /** “Create New” callback; when omitted, the row is hidden */
  onCreateNew?: () => void;
  /** Rich list items for the popover */
  menuItems?: MenuItem[];
  /** Optional sectioned menu */
  menuSections?: MenuSection[];
  /** Optional remove handler; when set we show the close icon */
  onRemove?: () => void;
  /** When true, popover opens immediately (used when picker is revealed) */
  autoOpen?: boolean;
  /** Notify parent once auto-open has been handled */
  onAutoOpenHandled?: () => void;
}) {
  const [anchorEl, setAnchorEl] = React.useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);
  const buttonRef = React.useRef<HTMLButtonElement | null>(null);

  React.useEffect(() => {
    if (autoOpen && !open && buttonRef.current) {
      setAnchorEl(buttonRef.current);
      onAutoOpenHandled?.();
    }
  }, [autoOpen, open, onAutoOpenHandled]);

  const handleClose = React.useCallback(() => {
    setAnchorEl(null);
  }, []);

  const handleSelect = React.useCallback(
    (label: string) => {
      onChange(label);
      setAnchorEl(null);
    },
    [onChange]
  );

  const valueMinHeight = Math.max(22, height - 22);

  return (
    <>
      {/* Trigger */}
      <Box
        sx={{
          width,
          height,
          padding: 1,
          // px: 1.25,
          // py: 1,
          borderRadius: 2,
          bgcolor: "action.hover",
          border: "1px solid #3d39391f",
          // borderColor: "divider",
          display: "flex",
          flexDirection: "column",
          justifyContent: "center",
          overflow: "hidden",
          position: "relative",
        }}
      >
        {onRemove && (
          <IconButton
            size="small"
            onClick={(event) => {
              event.stopPropagation();
              onRemove();
            }}
            sx={{
              position: "absolute",
              top: 4,
              right: 4,
              width: 20,
              height: 20,
              borderRadius: 1,
              color: "text.secondary",
              "&:hover": { bgcolor: "action.hover" },
            }}
          >
            <CloseRoundedIcon sx={{ fontSize: 16 }} />
          </IconButton>
        )}
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{
            fontSize: 11,
            lineHeight: 1,
            color: "#8d91a3",
            marginTop: 0.5,
            pr: onRemove ? 2.5 : 0,
          }}
        >
          {label}
        </Typography>

        <ButtonBase
          ref={buttonRef}
          onClick={(e) => setAnchorEl(e.currentTarget)}
          sx={{
            mt: 0.25,
            width: "100%",
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            textAlign: "left",
            borderRadius: 1,
            p: 0,
            minHeight: valueMinHeight,
            px: 0.25,
          }}
        >
          <Typography
            sx={{
              fontSize: 14,
              lineHeight: 1.2,
              fontWeight: 500,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {value}
          </Typography>
          <KeyboardArrowDownRoundedIcon fontSize="small" />
        </ButtonBase>
      </Box>

      <FieldPickerMenuPopover
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        onSelect={handleSelect}
        menuTitle={menuTitle}
        onCreateNew={
          onCreateNew
            ? () => {
                onCreateNew();
              }
            : undefined
        }
        menuItems={menuItems}
        menuSections={menuSections}
        currentValue={value}
      />
    </>
  );
}
