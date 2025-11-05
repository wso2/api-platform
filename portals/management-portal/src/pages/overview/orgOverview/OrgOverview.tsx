import React from "react";
import {
  Box,
  Grid,
  IconButton,
  Paper,
  Stack,
  Typography,
  Tooltip,
  Skeleton,
} from "@mui/material";
import AccessTimeRoundedIcon from "@mui/icons-material/AccessTimeRounded";
import type { Project } from "../../../hooks/projects";
import { Button } from "../../../components/src/components/Button";
import { AddIcon } from "../../../components/src/Icons/generated";
import { SearchBar } from "../../../components/src/components/SearchBar";
import CreateProjectForm from "./CreateProjectForm";
import ExploreMore from "./ExploreMore";
import {
  Card,
  CardActionArea,
  CardContent,
} from "../../../components/src/components/Card";
import Settings from "../../../components/src/Icons/generated/Settings";

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

const truncate = (text: string | undefined | null, max: number) => {
  const t = (text ?? "").trim();
  if (t.length <= max) return { shown: t, truncated: false };
  return { shown: t.slice(0, Math.max(0, max - 1)) + "…", truncated: true };
};

const SKELETON_COUNT = 8;

const OrgOverview: React.FC<OrgOverviewProps> = ({
  projects,
  onSelectProject,
  onCreateProject,
  onRefresh,
  loading,
}) => {
  const [query, setQuery] = React.useState("");
  const [createOpen, setCreateOpen] = React.useState(false);

  const openCreate = React.useCallback(() => {
    setCreateOpen(true);
  }, []);

  const filtered = React.useMemo(() => {
    if (!query.trim()) return projects;
    const q = query.toLowerCase();
    return projects.filter((p) => p.name.toLowerCase().includes(q));
  }, [projects, query]);

  const renderFooter = (p?: Project) => (
    <Box
      display="flex"
      justifyContent="space-between"
      alignItems="center"
      mt="auto"
    >
      <Box display="flex" flexDirection="row" gap={1} alignItems="center">
        <AccessTimeRoundedIcon fontSize="small" color="secondary" />
        <Typography variant="body2">
          {p?.createdAt ? getRelativeTime(p.createdAt) : "—"}
        </Typography>
      </Box>
      <Stack direction="row" justifyContent="flex-end">
        <IconButton
          size="small"
          onClick={(e) => {
            // prevent card click if you later add a menu here
            e.stopPropagation();
          }}
        >
          <Settings sx={{ fontSize: 16 }} />
        </IconButton>
      </Stack>
    </Box>
  );

  const renderSkeletonCards = () =>
    Array.from({ length: SKELETON_COUNT }).map((_, i) => (
      <Grid key={`sk-${i}`} size={{ xs: 12, sm: 6, md: 4, lg: 3 }}>
        <Card testId="project-card">
          <CardActionArea testId={"project-card-body"}>
            <CardContent
              sx={{
                py: 3,
                height: 175,
                display: "flex",
                flexDirection: "column",
                gap: 1,
              }}
            >
              <Stack direction="row" spacing={1} alignItems="center">
                <Skeleton variant="circular" width={42} height={42} />
                <Skeleton variant="text" width="60%" height={28} />
              </Stack>

              <Skeleton variant="text" width="90%" height={18} />
              <Skeleton variant="text" width="80%" height={18} />

              {/* footer area pinned to bottom */}
              <Box
                mt="auto"
                display="flex"
                justifyContent="space-between"
                alignItems="center"
              >
                <Box display="flex" gap={1} alignItems="center">
                  <Skeleton variant="circular" width={16} height={16} />
                  <Skeleton variant="text" width={80} height={16} />
                </Box>
                <Skeleton variant="circular" width={24} height={24} />
              </Box>
            </CardContent>
          </CardActionArea>
        </Card>
      </Grid>
    ));

  return (
    <Box>
      {/* Header (hidden in create mode) */}
      {!createOpen && (
        <Stack
          direction="row"
          alignItems="center"
          justifyContent="space-between"
          spacing={1}
        >
          <Stack direction="row" alignItems="center" spacing={1}>
            <Typography variant="h3" fontWeight={700}>
              All Projects
            </Typography>
          </Stack>

          <Stack direction="row" spacing={1}>
            <Button
              variant="contained"
              onClick={openCreate}
              disabled={!onCreateProject}
              startIcon={<AddIcon fontSize="small" />}
            >
              Create
            </Button>
          </Stack>
        </Stack>
      )}

      {/* Create Form */}
      {createOpen && (
        <CreateProjectForm
          onBack={() => setCreateOpen(false)}
          onSubmit={async (name, description) => {
            if (onCreateProject) {
              await onCreateProject(name, description);
              if (onRefresh) await onRefresh();
            }
            setCreateOpen(false);
          }}
        />
      )}

      {/* Search + List (hidden while creating) */}
      {!createOpen && (
        <>
          <Box sx={{ mt: 3, mb: 4 }}>
            <SearchBar
              testId="org-projects"
              placeholder="Search projects"
              inputValue={query}
              onChange={setQuery}
              iconPlacement="left"
              bordered
              size="medium"
              color="secondary"
            />
          </Box>

          <Grid container spacing={2} alignItems="stretch">
            {/* Skeletons while loading */}
            {loading && renderSkeletonCards()}

            {!loading &&
              filtered.map((project) => {
                const nameTr = truncate(project.name, 20);
                const descTr = truncate(project.description ?? "", 150);

                return (
                  <Grid key={project.id} size={{ xs: 12, sm: 6, md: 4, lg: 3 }}>
                    <Card testId="project-card">
                      <CardActionArea
                        onClick={() => onSelectProject(project)}
                        testId="project-card-body"
                        sx={{ height: "100%" }}
                      >
                        <CardContent
                          sx={{
                            py: 3,
                            height: 168,
                            display: "flex",
                            flexDirection: "column",
                            gap: 1,
                          }}
                        >
                          <Stack
                            direction="row"
                            spacing={1}
                            alignItems="center"
                            sx={{ minHeight: 42 }}
                          >
                            <Box
                              sx={{
                                width: 42,
                                height: 42,
                                borderRadius: "50%",
                                bgcolor: "#26af82",
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                fontWeight: 600,
                                fontSize: 18,
                                textTransform: "uppercase",
                                color: "white",
                                flex: "0 0 42px",
                              }}
                            >
                              {project.name.slice(0, 1)}
                            </Box>

                            {nameTr.truncated ? (
                              <Tooltip title={project.name} placement="top">
                                <Typography
                                  variant="h4"
                                  fontWeight={500}
                                  sx={{
                                    overflow: "hidden",
                                    textOverflow: "ellipsis",
                                    whiteSpace: "nowrap",
                                  }}
                                >
                                  {nameTr.shown}
                                </Typography>
                              </Tooltip>
                            ) : (
                              <Typography variant="h4" fontWeight={500} noWrap>
                                {nameTr.shown}
                              </Typography>
                            )}
                          </Stack>

                          {/* Description */}
                          {descTr.shown &&
                            (descTr.truncated ? (
                              <Tooltip
                                title={project.description}
                                placement="top"
                              >
                                <Typography
                                  variant="body1"
                                  color="#959595"
                                  sx={{
                                    overflow: "hidden",
                                    display: "-webkit-box",
                                    WebkitLineClamp: 2,
                                    WebkitBoxOrient: "vertical",
                                  }}
                                >
                                  {descTr.shown}
                                </Typography>
                              </Tooltip>
                            ) : (
                              <Typography
                                variant="body1"
                                color="#959595"
                                sx={{
                                  overflow: "hidden",
                                  display: "-webkit-box",
                                  WebkitLineClamp: 2,
                                  WebkitBoxOrient: "vertical",
                                }}
                              >
                                {descTr.shown}
                              </Typography>
                            ))}

                          {/* Footer pinned to bottom */}
                          {renderFooter(project)}
                        </CardContent>
                      </CardActionArea>
                    </Card>
                  </Grid>
                );
              })}

            {!loading && filtered.length === 0 && (
              <Grid size={{ xs: 12, sm: 6, md: 4, lg: 3 }}>
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

      {/* Explore More */}
      {!createOpen && <ExploreMore />}
    </Box>
  );
};

export default OrgOverview;
