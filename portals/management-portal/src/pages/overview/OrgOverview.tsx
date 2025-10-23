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
  Typography,
} from "@mui/material";
import SearchRoundedIcon from "@mui/icons-material/SearchRounded";
import InfoOutlinedIcon from "@mui/icons-material/InfoOutlined";
import LibraryBooksOutlinedIcon from "@mui/icons-material/LibraryBooksOutlined";
import SupportAgentOutlinedIcon from "@mui/icons-material/SupportAgentOutlined";
import SettingsRoundedIcon from "@mui/icons-material/SettingsRounded";
import AccessTimeRoundedIcon from "@mui/icons-material/AccessTimeRounded";

import type { Project } from "../../hooks/projects";
import { Button } from "../../components/src/components/Button";
import CreateProjectDialog from "../../components/CreateProjectDialog";

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
    links: [{ label: "Bijira Key Concepts", href: "#" }],
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

  const filtered = React.useMemo(() => {
    if (!query.trim()) return projects;
    const q = query.toLowerCase();
    return projects.filter((project) => project.name.toLowerCase().includes(q));
  }, [projects, query]);

  const handleCreate = React.useCallback(
    async (name: string, description: string) => {
      if (!onCreateProject) return;

      await onCreateProject(name, description);
      if (onRefresh) {
        await onRefresh();
      }
    },
    [onCreateProject, onRefresh]
  );

  return (
    <Box>
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
          {/* <IconButton size="small" onClick={onRefresh} disabled={loading}>
            <CachedRoundedIcon fontSize="small" />
          </IconButton> */}
        </Stack>

        <Stack direction="row" spacing={1}>
          {/* <IconButton size="small">
            <AppsRoundedIcon fontSize="small" />
          </IconButton>
          <IconButton size="small">
            <ViewAgendaRoundedIcon fontSize="small" />
          </IconButton> */}
          <Button
            variant="contained"
            onClick={() => setCreateOpen(true)}
            disabled={!onCreateProject}
          >
            + Create
          </Button>
        </Stack>
      </Stack>

      <TextField
        fullWidth
        placeholder="Search projects"
        value={query}
        onChange={(event) => setQuery(event.target.value)}
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
                minWidth: 300,
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
                    color="primary"
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

      {onCreateProject && (
        <CreateProjectDialog
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreate={async (name, description) => {
            await handleCreate(name, description);
            setCreateOpen(false);
          }}
          defaultName=""
        />
      )}
    </Box>
  );
};

export default OrgOverview;
