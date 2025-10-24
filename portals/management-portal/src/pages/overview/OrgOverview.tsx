// src/pages/OrgOverview.tsx
import React from "react";
import {
  Box,
  Card,
  CardActionArea,
  CardContent,
  Grid,
  IconButton,
  InputAdornment,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@mui/material";
import SearchRoundedIcon from "@mui/icons-material/SearchRounded";
import InfoOutlined from "@mui/icons-material/InfoOutlined";
import EditOutlined from "@mui/icons-material/EditOutlined";
import SettingsRoundedIcon from "@mui/icons-material/SettingsRounded";
import AccessTimeRoundedIcon from "@mui/icons-material/AccessTimeRounded";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import LibraryBooksOutlinedIcon from "@mui/icons-material/LibraryBooksOutlined";
import SupportAgentOutlinedIcon from "@mui/icons-material/SupportAgentOutlined";

import type { Project } from "../../hooks/projects";
import { Button } from "../../components/src/components/Button";
import { slugify } from "../../utils/slug";

type OrgOverviewProps = {
  projects: Project[];
  onSelectProject: (project: Project) => void;
  onCreateProject?: (name: string, description: string) => Promise<void>;
  onRefresh?: () => Promise<void> | void;
  loading?: boolean;
};

const getRelativeTime = (isoDate: string): string => {
  const date = new Date(isoDate);
  if (Number.isNaN(date.getTime())) return "—";
  const now = new Date();
  const diffMs = date.getTime() - now.getTime();
  const abs = Math.abs(diffMs);

  const minute = 60 * 1000;
  const hour = 60 * minute;
  const day = 24 * hour;

  const rtf = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

  if (abs < hour) {
    const minutes = Math.round(diffMs / minute);
    return rtf.format(minutes, "minute");
  }
  if (abs < day) {
    const hours = Math.round(diffMs / hour);
    return rtf.format(hours, "hour");
  }
  const days = Math.round(diffMs / day);
  return rtf.format(days, "day");
};

const defaultLearningLinks = [
  {
    icon: <InfoOutlinedIcon sx={{ color: "#1f51ff" }} />,
    title: "Tutorials",
    links: [
      { label: "Create a REST API from an OpenAPI Definition", href: "#" },
      { label: "Engage Access Control to the API", href: "#" },
      { label: "Engage API policies to the API", href: "#" },
    ],
  },
  {
    icon: <LibraryBooksOutlinedIcon sx={{ color: "#0d99ff" }} />,
    title: "References",
    links: [{ label: "API Platform Key Concepts", href: "#" }],
  },
  {
    icon: <SupportAgentOutlinedIcon sx={{ color: "#8854d0" }} />,
    title: "Support",
    links: [{ label: "Get Support on Discord", href: "#" }],
  },
];

const OrgOverview: React.FC<OrgOverviewProps> = ({
  projects,
  onSelectProject,
  onCreateProject,
  onRefresh,
}) => {
  const [query, setQuery] = React.useState("");
  const [createOpen, setCreateOpen] = React.useState(false);

  // Inline create form state
  const [displayName, setDisplayName] = React.useState("");
  const [identifier, setIdentifier] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [identifierLocked, setIdentifierLocked] = React.useState(true);
  const [errors, setErrors] = React.useState<{ name?: string; id?: string }>(
    {}
  );
  const [submitting, setSubmitting] = React.useState(false);

  const openCreate = React.useCallback(() => {
    setCreateOpen(true);
    setDisplayName("");
    setIdentifier("");
    setDescription("");
    setIdentifierLocked(true);
    setErrors({});
    setSubmitting(false);
  }, []);

  const cancelCreate = React.useCallback(() => {
    setCreateOpen(false);
  }, []);

  React.useEffect(() => {
    if (identifierLocked) {
      setIdentifier(slugify(displayName));
    }
  }, [displayName, identifierLocked]);

  const validate = React.useCallback(() => {
    const next: { name?: string; id?: string } = {};
    if (!displayName.trim()) next.name = "Display name is required";
    if (!identifier.trim()) next.id = "Identifier is required";
    setErrors(next);
    return Object.keys(next).length === 0;
  }, [displayName, identifier]);

  const handleSubmit = React.useCallback(async () => {
    if (!onCreateProject) return;
    if (!validate()) return;

    setSubmitting(true);
    try {
      await onCreateProject(displayName.trim(), description.trim());
      if (onRefresh) await onRefresh();
      setCreateOpen(false);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create project";
      setErrors((prev) => ({ ...prev, id: message }));
    } finally {
      setSubmitting(false);
    }
  }, [onCreateProject, onRefresh, displayName, description, validate]);

  const filtered = React.useMemo(() => {
    if (!query.trim()) return projects;
    const q = query.toLowerCase();
    return projects.filter((p) => p.name.toLowerCase().includes(q));
  }, [projects, query]);

  return (
    <Box>
      {/* Header (hidden in create mode) */}
      {!createOpen && (
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          spacing={2}
        >
          <Stack direction="row" alignItems="center" spacing={1}>
            <Typography variant="h5" fontWeight={700}>
              All Projects
            </Typography>
          </Stack>

          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              onClick={openCreate}
              disabled={!onCreateProject}
            >
              + Create
            </Button>
          </Stack>
        </Stack>
      )}

      {/* Inline Create Form */}
      {createOpen && (
        <Box
          sx={{
            mb: 4,
            p: 3,
          }}
        >
          <Typography variant="subtitle1" fontWeight={600} gutterBottom>
            Create a Project
          </Typography>

          {/* ROW 1: Display Name + Identifier (each 50%) */}
         <Box sx={{ mt: 2, width: { xs: "100%", md: "50%" } }} display={"flex"} gap={2} flexDirection={"row"}>
            <TextField
              label="Display Name"
              placeholder="Enter Project Name"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              fullWidth
              error={Boolean(errors.name)}
              helperText={errors.name}
            />

            <TextField
              label="Identifier"
              placeholder="auto-generated"
              value={identifier}
              onChange={(e) => {
                if (identifierLocked) return;
                setIdentifier(slugify(e.target.value));
              }}
              fullWidth
              error={Boolean(errors.id)}
              helperText={errors.id || ""}
              InputProps={{
                readOnly: identifierLocked,
                endAdornment: (
                  <Tooltip title={identifierLocked ? "Unlock to edit" : "Lock"}>
                    <IconButton
                      aria-label="Toggle identifier edit"
                      onClick={() => setIdentifierLocked((p) => !p)}
                      size="small"
                    >
                      <EditOutlined fontSize="small" />
                    </IconButton>
                  </Tooltip>
                ),
              }}
            />
          </Box>

          {/* ROW 2: Description (50%) */}
          <Box sx={{ mt: 3, width: { xs: "100%", md: "50%" } }}>
            <TextField
              label="Description"
              placeholder="Enter Description here"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              fullWidth
              multiline
              minRows={1}
              maxRows={4}
              // helperText="Optional"
            />
          </Box>

          {/* Buttons under the form */}
          <Stack direction="row" spacing={1} justifyContent="flex-start" mt={4}>
            <Button
              variant="outlined"
              onClick={cancelCreate}
              disabled={submitting}
            >
              Cancel
            </Button>
            <Button
              variant="contained"
              onClick={handleSubmit}
              disabled={submitting || !displayName.trim() || !identifier.trim()}
            >
              Create
            </Button>
          </Stack>
        </Box>
      )}

      {/* Search + List (hidden while creating) */}
      {!createOpen && (
        <>
          <TextField
            fullWidth
            placeholder="Search projects"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            sx={{ mt: 3, mb: 4 }}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <SearchRoundedIcon fontSize="small" />
                </InputAdornment>
              ),
              sx: {
                borderRadius: 2,
                bgcolor: "background.paper",
              },
            }}
          />

          <Grid container spacing={2}>
            {filtered.map((project) => (
              <Grid key={project.id}>
                <Card
                  elevation={0}
                  sx={{
                    borderRadius: 3,
                    border: "1px solid",
                    borderColor: "divider",
                    bgcolor: "background.paper",
                    minWidth: 328,
                  }}
                >
                  <CardActionArea
                    onClick={() => onSelectProject(project)}
                    sx={{ borderRadius: 3, height: "100%" }}
                  >
                    <CardContent>
                      <Stack spacing={2}>
                        <Stack direction="row" spacing={1} alignItems="center">
                          <Box
                            sx={{
                              width: 42,
                              height: 42,
                              borderRadius: "50%",
                              bgcolor: "#26af82",
                              display: "flex",
                              alignItems: "center",
                              justifyContent: "center",
                              color: "primary.contrastText",
                              fontWeight: 600,
                              fontSize: 18,
                              textTransform: "uppercase",
                            }}
                          >
                            {project.name.slice(0, 1)}
                          </Box>
                          <Typography variant="h6" fontWeight={500} noWrap>
                            {project.name}
                          </Typography>
                        </Stack>

                        <Stack
                          direction="row"
                          spacing={1}
                          alignItems="center"
                          color="text.secondary"
                        >
                          <AccessTimeRoundedIcon fontSize="small" />
                          <Typography variant="body2">
                            {project.createdAt
                              ? getRelativeTime(project.createdAt)
                              : "—"}
                          </Typography>
                        </Stack>

                        <Stack direction="row" justifyContent="flex-end">
                          <IconButton size="small">
                            <SettingsRoundedIcon fontSize="small" />
                          </IconButton>
                        </Stack>
                      </Stack>
                    </CardContent>
                  </CardActionArea>
                </Card>
              </Grid>
            ))}

            {filtered.length === 0 && (
              <Grid>
                <Paper
                  variant="outlined"
                  sx={{
                    p: 6,
                    textAlign: "center",
                    borderRadius: 3,
                    color: "text.secondary",
                  }}
                >
                  <Typography variant="h6" fontWeight={600} gutterBottom>
                    No projects found
                  </Typography>
                  <Typography variant="body2">
                    Try adjusting your search or create a new project.
                  </Typography>
                </Paper>
              </Grid>
            )}
          </Grid>
        </>
      )}

      {/* Explore More (keep code EXACTLY as you provided; only hidden while creating) */}
      {!createOpen && (
        <Box mt={6}>
          <Typography variant="h6" fontWeight={600} mb={2}>
            Explore More
          </Typography>

          <Paper
            variant="outlined"
            sx={{
              borderRadius: 3,
              p: 3,
              display: "grid",
              gap: 3,
              gridTemplateColumns: { xs: "1fr", md: "repeat(3, 1fr)" },
            }}
          >
            {defaultLearningLinks.map((section) => (
              <Stack key={section.title} spacing={1.5}>
                <Stack direction="row" spacing={1} alignItems="center">
                  {section.icon}
                  <Typography fontWeight={600}>{section.title}</Typography>
                </Stack>
                <Stack spacing={1}>
                  {section.links.map((link) => (
                    <Typography
                      key={link.label}
                      variant="body2"
                      color="#252525ff"
                      component="a"
                      sx={{ textDecoration: "none", cursor: "pointer" }}
                      href={link.href}
                      onClick={(event) => event.preventDefault()}
                    >
                      → {link.label}
                    </Typography>
                  ))}
                </Stack>
              </Stack>
            ))}
          </Paper>
        </Box>
      )}
    </Box>
  );
};

export default OrgOverview;
