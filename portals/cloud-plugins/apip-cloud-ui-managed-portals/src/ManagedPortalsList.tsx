/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useMemo, useState } from "react";
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  Grid,
  IconButton,
  InputAdornment,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ExternalLink,
  Pencil,
  Plus,
  Search,
  Trash2,
} from "@wso2/oxygen-ui-icons-react";

import { useManagedPortalList } from "./hooks";
import type { ManagedPortal } from "./types";
import { PortalIllustration } from "./PortalIllustration";

const devportalLogo = new URL("./images/devportal-logo.png", import.meta.url)
  .href;

export type ManagedPortalsListProps = {
  /** Switches parent to the create view; create is a full page, not a modal. */
  onCreate: () => void;
  /** Switches parent to the edit view for the given portal; edit is a full page, not a modal. */
  onEdit: (portal: ManagedPortal) => void;
};

/**
 * Short relative-time formatter local to this feature so the package stays
 * dependency-free. Picks the coarsest unit that fits a portal card ("3h ago",
 * "5d ago", "2mo ago"). Clock skew that puts the stamp in the future collapses
 * to "just now" rather than the misleading "3h ago".
 */
function shortRelative(iso: string | null | undefined): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const seconds = Math.round((Date.now() - then) / 1000);
  if (seconds < 60) return "just now";
  if (seconds < 3600) return `${Math.round(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.round(seconds / 3600)}h ago`;
  if (seconds < 2592000) return `${Math.round(seconds / 86400)}d ago`;
  if (seconds < 31536000) return `${Math.round(seconds / 2592000)}mo ago`;
  return `${Math.round(seconds / 31536000)}y ago`;
}

export default function ManagedPortalsList({
  onCreate,
  onEdit,
}: ManagedPortalsListProps) {
  const { portals, isLoading, error, remove } = useManagedPortalList();
  const [deleteTarget, setDeleteTarget] = useState<ManagedPortal | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [searchQuery, setSearchQuery] = useState("");

  const filteredPortals = useMemo(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return portals;
    return portals.filter(
      (portal) =>
        portal.name.toLowerCase().includes(q) ||
        portal.handle.toLowerCase().includes(q) ||
        (portal.description?.toLowerCase().includes(q) ?? false) ||
        (portal.url?.toLowerCase().includes(q) ?? false),
    );
  }, [portals, searchQuery]);

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || deleting) return;
    setDeleting(true);
    try {
      await remove(deleteTarget.id);
      setDeleteTarget(null);
    } catch {
      // Hook already notified.
    } finally {
      setDeleting(false);
    }
  };

  return (
    <PageContent fullWidth>
      <Grid container spacing={2} sx={{ width: "100%", m: 0 }}>
        <Grid size={{ xs: 12 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "flex-start",
              justifyContent: "space-between",
              flexWrap: "nowrap",
              gap: 2,
            }}
          >
            <PageTitle sx={{ minWidth: 0, flex: 1 }}>
              <PageTitle.Header>Portals</PageTitle.Header>
              <PageTitle.SubHeader>
                Manage the portals for this organization.
              </PageTitle.SubHeader>
            </PageTitle>

            <Stack
              direction="row"
              spacing={1.5}
              sx={{ ml: "auto", flexShrink: 0 }}
            >
              {portals.length > 0 ? (
                <Button
                  variant="contained"
                  onClick={onCreate}
                  startIcon={<Plus size={20} />}
                >
                  Add Portal
                </Button>
              ) : null}
            </Stack>
          </Box>
        </Grid>

        {isLoading ? (
          <Grid size={{ xs: 12 }}>
            <Typography variant="body2" color="text.secondary">
              Loading portals…
            </Typography>
          </Grid>
        ) : error ? (
          <Grid size={{ xs: 12 }}>
            <Typography variant="body2" color="error">
              {error.message}
            </Typography>
          </Grid>
        ) : portals.length === 0 ? (
          <Grid size={{ xs: 12 }}>
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                minHeight: "50vh",
                px: 3,
                py: 6,
              }}
            >
              <Stack
                spacing={1}
                alignItems="center"
                sx={{ maxWidth: 440, textAlign: "center" }}
              >
                <PortalIllustration />
                <Typography variant="h5" sx={{ fontWeight: 700, pt: 2 }}>
                  Create your first portal
                </Typography>
                <Typography variant="body1" color="text.secondary" sx={{ opacity: 0.7 }}>
                  Set up a developer portal to help developers discover your APIs
                  and get started with your services.
                </Typography>
                <Box sx={{ pt: 2 }}>
                  <Button
                    variant="contained"
                    onClick={onCreate}
                    startIcon={<Plus size={20} />}
                  >
                    Create Portal
                  </Button>
                </Box>
              </Stack>
            </Box>
          </Grid>
        ) : (
          <>
            <Grid size={{ xs: 12 }}>
              <TextField
                fullWidth
                size="medium"
                placeholder="Search Portals..."
                value={searchQuery}
                onChange={(event) => setSearchQuery(event.target.value)}
                slotProps={{
                  htmlInput: { "aria-label": "Search portals" },
                  input: {
                    startAdornment: (
                      <InputAdornment position="start">
                        <Search size={20} />
                      </InputAdornment>
                    ),
                  },
                }}
              />
            </Grid>

            <Grid size={{ xs: 12 }}>
              {filteredPortals.length === 0 ? (
                <Typography variant="body2" color="text.secondary">
                  No portals found.
                </Typography>
              ) : (
                <Box
                  sx={{
                    display: "grid",
                    gridTemplateColumns: {
                      xs: "1fr",
                      md: "repeat(2, minmax(0, 1fr))",
                      xl: "repeat(3, minmax(0, 1fr))",
                    },
                    gap: 3,
                  }}
                >
                  {filteredPortals.map((portal) => {
                    const status = portal.status ?? "active";
                    const canVisit = status === "active" && Boolean(portal.url);
                    const isPending = status === "pending";
                    return (
                      <Card
                        key={portal.id}
                        variant="outlined"
                        sx={{ display: "flex", minWidth: 0 }}
                      >
                        <CardContent
                          sx={{
                            p: 2.5,
                            "&:last-child": { pb: 2.5 },
                            display: "flex",
                            flexDirection: "column",
                            gap: 2,
                            flex: 1,
                            minWidth: 0,
                          }}
                        >
                          <Box
                            sx={{
                              display: "flex",
                              alignItems: "flex-start",
                              gap: 2,
                              flex: 1,
                            }}
                          >
                            <Box
                              sx={{
                                width: { xs: 64, sm: 72 },
                                height: { xs: 64, sm: 72 },
                                flexShrink: 0,
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "center",
                                border: 1,
                                borderColor: "divider",
                                borderRadius: 1,
                                bgcolor: "action.hover",
                                opacity: isPending ? 0.4 : 1,
                              }}
                            >
                              <Box
                                component="img"
                                src={devportalLogo}
                                alt=""
                                sx={{
                                  width: "75%",
                                  height: "75%",
                                  objectFit: "contain",
                                }}
                              />
                            </Box>
                            <Stack
                              spacing={0.5}
                              sx={{ flex: 1, minWidth: 0, pt: 0.5 }}
                            >
                              <Typography
                                variant="h6"
                                sx={{ fontWeight: 600, overflowWrap: "anywhere" }}
                              >
                                {portal.name}
                              </Typography>
                              <Typography
                                variant="body2"
                                color="text.secondary"
                                sx={{
                                  overflowWrap: "anywhere",
                                  display: "-webkit-box",
                                  WebkitBoxOrient: "vertical",
                                  WebkitLineClamp: 2,
                                  overflow: "hidden",
                                  lineHeight: 1.5,
                                  minHeight: "3em",
                                }}
                              >
                                {portal.description || ""}
                              </Typography>
                            </Stack>
                            <Tooltip
                              title={
                                isPending
                                  ? "Editing is available once provisioning completes."
                                  : status === "failed"
                                    ? "Editing is disabled while the portal is in a failed state. Delete and re-create."
                                    : "Edit portal"
                              }
                              arrow
                            >
                              <span>
                                <IconButton
                                  size="small"
                                  aria-label={`Edit ${portal.name}`}
                                  onClick={() => onEdit(portal)}
                                  disabled={status !== "active"}
                                >
                                  <Pencil size={16} />
                                </IconButton>
                              </span>
                            </Tooltip>
                          </Box>

                          <Stack
                            spacing={2}
                            sx={{ borderTop: 1, borderColor: "divider", pt: 2 }}
                          >
                            <Box
                              sx={{
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "space-between",
                                gap: 2,
                              }}
                            >
                              <Typography variant="body1" color="text.secondary">
                                Login environment
                              </Typography>
                              {portal.loginEnvironment ? (
                                <Chip
                                  label={portal.loginEnvironment}
                                  size="small"
                                  variant="outlined"
                                  sx={{ maxWidth: "55%" }}
                                />
                              ) : (
                                <Typography
                                  variant="body1"
                                  color="text.secondary"
                                >
                                  -
                                </Typography>
                              )}
                            </Box>
                            <Box
                              sx={{
                                display: "flex",
                                alignItems: "center",
                                justifyContent: "space-between",
                                gap: 2,
                              }}
                            >
                              <Typography variant="body1" color="text.secondary">
                                Updated
                              </Typography>
                              <Typography variant="body1">
                                {shortRelative(portal.updatedAt) || "-"}
                              </Typography>
                            </Box>
                          </Stack>

                          <Stack direction="row" spacing={1}>
                            {canVisit ? (
                              <Button
                                fullWidth
                                size="small"
                                sx={{ height: 36, minHeight: 36, py: 0.5 }}
                                variant="contained"
                                component="a"
                                href={portal.url}
                                target="_blank"
                                rel="noopener noreferrer"
                                startIcon={<ExternalLink size={16} />}
                              >
                                Open Portal
                              </Button>
                            ) : (
                              <Button
                                fullWidth
                                size="small"
                                sx={{ height: 36, minHeight: 36, py: 0.5 }}
                                variant="contained"
                                disabled
                                startIcon={
                                  isPending ? (
                                    <CircularProgress size={16} color="inherit" />
                                  ) : undefined
                                }
                              >
                                {isPending
                                  ? "Creating portal..."
                                  : status === "failed"
                                    ? "Creation failed"
                                    : "Portal unavailable"}
                              </Button>
                            )}
                            {/* Keep deletion available so pending or failed portals can be recovered. */}
                            <Tooltip title="Delete portal" arrow>
                              <IconButton
                                size="small"
                                color="error"
                                aria-label={`Delete ${portal.name}`}
                                onClick={() => setDeleteTarget(portal)}
                                sx={{
                                  width: 36,
                                  height: 36,
                                  flexShrink: 0,
                                }}
                              >
                                <Trash2 size={16} />
                              </IconButton>
                            </Tooltip>
                          </Stack>
                        </CardContent>
                      </Card>
                    );
                  })}
                </Box>
              )}
            </Grid>
          </>
        )}
      </Grid>

      <Dialog
        open={Boolean(deleteTarget)}
        onClose={deleting ? undefined : () => setDeleteTarget(null)}
      >
        <DialogTitle>Delete Portal</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to delete {deleteTarget?.name}?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => setDeleteTarget(null)}
            variant="outlined"
            color="secondary"
            disabled={deleting}
          >
            Cancel
          </Button>
          <Button
            color="error"
            onClick={handleDeleteConfirm}
            disabled={deleting}
            startIcon={
              deleting ? (
                <CircularProgress size={16} color="inherit" />
              ) : undefined
            }
          >
            {deleting ? "Deleting…" : "Delete"}
          </Button>
        </DialogActions>
      </Dialog>
    </PageContent>
  );
}
