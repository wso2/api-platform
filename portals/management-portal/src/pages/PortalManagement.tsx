// src/pages/PortalManagement.tsx
import * as React from "react";
import {
  Box,
  Grid,
  Stack,
  Typography,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  TextField,
  Snackbar,
  Alert,
  Card,
  CardActionArea,
  CardContent,
} from "@mui/material";
import { Button } from "../components/src/components/Button";

import AiBanner from "./AITheming.svg";
import BijiraDPLogo from "./BijiraDPLogo.png";
import NewDP from "./undraw_windows_kqsk.svg";

import PortalCard from "./portals/PortalCard";
import PortalForm from "./portals/PortalForm";
import { PrivatePreview, PublicPreview } from "./portals/PortalPreviews";
import PromoBanner from "./portals/PromoBanner";
import ThemeSettingsPanel from "./portals/ThemeSettingsPanel";

type PortalType = "private" | "public";
type Mode = "list" | "form" | "theme";

const slugify = (s: string) =>
  s
    .toLowerCase()
    .trim()
    .replace(/['"]/g, "")
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");

const PortalManagement: React.FC = () => {
  const [portalType] = React.useState<PortalType>("public");

  // Card content
  const [cardTitle, setCardTitle] = React.useState("Default Developer Portal");
  const [cardDesc, setCardDesc] = React.useState(
    "Lorem Ipsum is simply dummy text of the printing and typesetting industry..."
  );

  // UI mode
  const [mode, setMode] = React.useState<Mode>("list");

  // Form state
  const [portalName, setPortalName] = React.useState<string>("");
  const [portalIdentifier, setPortalIdentifier] = React.useState<string>("");
  const [portalDomains, setPortalDomains] = React.useState<string>("");

  // Edit modal + snackbar
  const [editOpen, setEditOpen] = React.useState(false);
  const [editTitle, setEditTitle] = React.useState(cardTitle);
  const [editDesc, setEditDesc] = React.useState(cardDesc);
  const [snack, setSnack] = React.useState<{ open: boolean; msg: string }>({
    open: false,
    msg: "",
  });

  const handleOpenEdit = () => {
    setEditTitle(cardTitle);
    setEditDesc(cardDesc);
    setEditOpen(true);
  };

  const handleSaveEdit = () => {
    setCardTitle(editTitle.trim() || cardTitle);
    setCardDesc(editDesc.trim());
    setEditOpen(false);
    setSnack({ open: true, msg: "Portal details updated successfully." });
  };

  const handleNameChange = (v: string) => {
    setPortalName(v);
    setPortalIdentifier(slugify(v));
  };
  const handleIdentifierChange = (v: string) => setPortalIdentifier(slugify(v));

  const handleActivate = () => {
    setSnack({ open: true, msg: "Developer Portal activated." });
    setMode("theme"); // switch to Theme Settings view
  };

  return (
    <Box sx={{ overflowX: "auto" }}>
      {/* Header (always shown) */}
      <Box>
        <Typography variant="h3" fontWeight={700}>
          {mode === "theme" ? "Theme Settings" : "Developer Portals"}
        </Typography>

        <Typography variant="body2" sx={{ mt: 0.5, mb: 3, maxWidth: 760 }}>
          {mode === "theme"
            ? "Manage and customize the theme settings for your organization."
            : "Define visibility of your portal and publish your first API. You can modify your selections later."}
        </Typography>
      </Box>

      {/* BODY */}
      {mode === "list" && (
        // Only the cards row
        <Grid container spacing={2} ml={1} mb={1}>
          <Grid>
            <PortalCard
              title={cardTitle}
              description={cardDesc}
              selected
              onClick={() => {}}
              logoSrc={BijiraDPLogo}
              userAuthLabel="Asgardeo Thunder"
              authStrategyLabel="Auth-Key"
              visibilityLabel="Private"
              onEdit={handleOpenEdit}
              onActivate={handleActivate} // go to theme view
            />
          </Grid>

          {/* Add New Developer Portal card */}
          <Grid>
            <Card
              variant="outlined"
              sx={{
                minHeight: 365, maxHeight: 363,
                borderRadius: 2,
                borderColor: "divider",
                maxWidth:400
              }}
            >
              <CardContent sx={{ p: 3, height: '100%' , minHeight:350, display: 'flex' }} >
                <Stack
                  spacing={3}
                  alignItems="center"
                  justifyContent="center"
                  display={'flex'}
                >
                  <Box
                    component="img"
                    src={NewDP}
                    alt="Add developer portal"
                    sx={{ width: 150, maxWidth: "100%", display: "block" }}
                  />

                  <Typography
                    align="center"
                    color="text.secondary"
                    sx={{ maxWidth: 520 }}
                  >
                    Lorem Ipsum is simply dummy text of the printing and
                    typesetting industry.
                  </Typography>

                  <Button fullWidth onClick={() => setMode("form")}>
                    Add Your New Developer Portal
                  </Button>
                </Stack>
              </CardContent>
            </Card>
          </Grid>
        </Grid>
      )}

      {mode === "form" && (
        // Show ONLY the form
        <Box sx={{ width: 560, maxWidth: "100%" }}>
          <PortalForm
            portalName={portalName}
            portalIdentifier={portalIdentifier}
            portalDomains={portalDomains}
            onChangeName={handleNameChange}
            onChangeIdentifier={handleIdentifierChange}
            onChangeDomains={setPortalDomains}
            onContinue={() => setMode("theme")}
            canContinue={!!portalName.trim()}
          />
        </Box>
      )}

      {mode === "theme" && (
        // Theme view: left (banner + settings) + right (preview)
        <Grid
          container
          columnSpacing={3}
          alignItems="flex-start"
          sx={{ flexWrap: "nowrap", minWidth: 960 }}
        >
          <Grid>
            <Stack spacing={2.5}>
              <PromoBanner imageSrc={AiBanner} onPrimary={() => {}} />
              <ThemeSettingsPanel />
            </Stack>
          </Grid>

          {/* Right column (preview) */}
          <Grid>
            <Box sx={{ position: "sticky", top: 24 }}>
              {portalType === "private" ? (
                <PrivatePreview />
              ) : (
                <PublicPreview />
              )}
            </Box>

            {/* Publish Theme button in theme view */}
            <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 2 }}>
              <Button
                variant="contained"
                onClick={() => console.log("Publish theme clicked")}
                sx={{
                  backgroundColor: "#FE8C3A",
                  "&:hover": { backgroundColor: "#e67d33" },
                }}
              >
                Publish Theme
              </Button>
            </Box>
          </Grid>
        </Grid>
      )}

      {/* Edit modal */}
      <Dialog
        open={editOpen}
        onClose={() => setEditOpen(false)}
        fullWidth
        maxWidth="sm"
      >
        <DialogTitle>Edit portal details</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={2} sx={{ mt: 1 }}>
            <TextField
              label="Title"
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              fullWidth
            />
            <TextField
              label="Description"
              value={editDesc}
              onChange={(e) => setEditDesc(e.target.value)}
              fullWidth
              multiline
              minRows={3}
            />
          </Stack>
        </DialogContent>
        <DialogActions>
          <Button variant="outlined" onClick={() => setEditOpen(false)}>
            Cancel
          </Button>
          <Button variant="contained" onClick={handleSaveEdit}>
            Save
          </Button>
        </DialogActions>
      </Dialog>

      {/* Snackbar */}
      <Snackbar
        open={snack.open}
        autoHideDuration={2500}
        onClose={() => setSnack((s) => ({ ...s, open: false }))}
        anchorOrigin={{ vertical: "bottom", horizontal: "right" }}
      >
        <Alert
          onClose={() => setSnack((s) => ({ ...s, open: false }))}
          severity="success"
          sx={{ width: "100%" }}
        >
          {snack.msg}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default PortalManagement;
