// src/pages/PortalManagement.tsx
import * as React from "react";
import {
  Box,
  Grid,
  Typography,
  Card,
  CardActionArea,
  CardContent,
  Stack,
  Chip,
  Divider,
  Radio,
  FormControlLabel,
  Paper,
  FormControl,
  TextField,
  Tabs,
  Tab,
  Switch,
  Accordion,
  AccordionSummary,
  AccordionDetails,
  InputAdornment,
} from "@mui/material";
import LockOutlinedIcon from "@mui/icons-material/LockOutlined";
import PublicOutlinedIcon from "@mui/icons-material/PublicOutlined";
import UploadFileOutlinedIcon from "@mui/icons-material/UploadFileOutlined";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import { Button } from "../components/src/components/Button";
import AiBanner from "./AITheming.svg";


type PortalType = "private" | "public";
type Step = "setup" | "theme";

const OptionCard: React.FC<{
  title: string;
  description: string;
  icon: React.ReactNode;
  selected: boolean;
  onClick: () => void;
}> = ({ title, description, icon, selected, onClick }) => (
  <Card
    variant="outlined"
    sx={{
      borderColor: selected ? "#069668" : "divider",
      boxShadow: selected ? 3 : 0,
      transition: "all 120ms ease",
      height: "100%",
      maxWidth: 470,
      minWidth: 470,
    }}
  >
    <CardActionArea onClick={onClick} sx={{ height: "100%" }}>
      <CardContent sx={{ display: "flex", gap: 2, alignItems: "flex-start" }}>
        <Box
          sx={{
            p: 1,
            borderRadius: 2,
            bgcolor: selected ? "#069668" : "action.hover",
            color: selected ? "primary.contrastText" : "text.secondary",
            display: "inline-flex",
          }}
        >
          {icon}
        </Box>
        <Box sx={{ flex: 1 }}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Typography variant="subtitle1">{title}</Typography>
            {selected && <Chip size="small" label="Selected" color="success" />}
          </Stack>
          <Typography fontSize={12} color="text.secondary" sx={{ mt: 0.5 }}>
            {description}
          </Typography>
        </Box>
        <FormControlLabel
          sx={{ m: 0, ml: "auto" }}
          control={
            <Radio
              checked={selected}
              sx={{
                color: "#069668",
                "&.Mui-checked": { color: "#069668" },
                "& .MuiTouchRipple-child": { backgroundColor: "#069668" },
              }}
            />
          }
          label=""
          onChange={onClick}
        />
      </CardContent>
    </CardActionArea>
  </Card>
);

const PrivatePreview: React.FC = () => (
  <Paper variant="outlined" sx={{ p: 3, minHeight: 560 }}>
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <LockOutlinedIcon fontSize="small" />
        <Typography variant="h6">Internal Marketplace</Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 520 }}>
        Here you will have some good context in the subheading for your
        developer portal so users can know more about your product
      </Typography>
      <Box sx={{ height: 160, borderRadius: 2, bgcolor: "action.hover" }} />
      <Stack direction="row" spacing={2} justifyContent="center">
        {["logo1", "logo2", "logo3", "logo4", "logo5"].map((k) => (
          <Box
            key={k}
            sx={{
              width: 64,
              height: 16,
              bgcolor: "action.hover",
              borderRadius: 1,
            }}
          />
        ))}
      </Stack>
      <Divider />
      <Typography variant="h6">Get started</Typography>
      <Stack direction={{ xs: "column", sm: "row" }} spacing={2}>
        <Box sx={{ flex: 1 }}>
          <Typography variant="subtitle1">
            Some guide title over here
          </Typography>
          <Typography variant="body2" color="text.secondary">
            A brief description for your guides. API greenfield, cache,
            container abstraction…
          </Typography>
          <Button size="small" sx={{ mt: 1 }}>
            Read more
          </Button>
        </Box>
        <Box
          sx={{
            flex: 1,
            height: 120,
            borderRadius: 2,
            bgcolor: "action.hover",
          }}
        />
      </Stack>
      <Stack spacing={2}>
        <Box sx={{ height: 140, borderRadius: 2, bgcolor: "action.hover" }} />
        <Box>
          <Typography variant="subtitle1">
            Another title for your guide
          </Typography>
          <Typography variant="body2" color="text.secondary">
            A brief description for your guides. API greenfield, container
            abstraction, etc.
          </Typography>
          <Button size="small" sx={{ mt: 1 }}>
            Read more
          </Button>
        </Box>
      </Stack>
      <Divider />
      <Typography variant="h6">Explore APIs</Typography>
      <Stack direction="row" spacing={2}>
        {[1, 2, 3].map((i) => (
          <Box
            key={i}
            sx={{
              flex: 1,
              height: 120,
              borderRadius: 2,
              bgcolor: "action.hover",
            }}
          />
        ))}
      </Stack>
    </Stack>
  </Paper>
);

const PublicPreview: React.FC = () => (
  <Paper variant="outlined" sx={{ p: 3, minHeight: 560 }}>
    <Stack spacing={2}>
      <Stack direction="row" alignItems="center" spacing={1}>
        <PublicOutlinedIcon fontSize="small" />
        <Typography variant="h6">Dev Portal</Typography>
      </Stack>
      <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 520 }}>
        Anyone with the link can view your portal and APIs. Great for open docs
        and public discovery.
      </Typography>
      <Box sx={{ height: 160, borderRadius: 2, bgcolor: "action.hover" }} />
      <Divider />
      <Typography variant="h6">Explore APIs</Typography>
      <Stack direction="row" spacing={2}>
        {[1, 2, 3].map((i) => (
          <Box
            key={i}
            sx={{
              flex: 1,
              height: 120,
              borderRadius: 2,
              bgcolor: "action.hover",
            }}
          />
        ))}
      </Stack>
    </Stack>
  </Paper>
);

/** ------- THEME UI (Step 2) ------- */

const Swatch = ({ value }: { value: string }) => (
  <Box
    sx={{
      width: 36,
      height: 36,
      borderRadius: 1,
      border: (t) => `1px solid ${t.palette.divider}`,
      bgcolor: value || "transparent",
    }}
  />
);

const ColorField: React.FC<{
  label: string;
  value: string;
  onChange: (v: string) => void;
  width?: number | string;
}> = ({ label, value, onChange, width = 220 }) => (
  <TextField
    label={label}
    value={value}
    onChange={(e) => onChange(e.target.value)}
    placeholder="#FFFFFF"
    size="small"
    sx={{ maxWidth: width }}
    InputProps={{
      endAdornment: (
        <InputAdornment position="end">
          <Swatch value={value} />
        </InputAdornment>
      ),
    }}
  />
);

// --- PromoBanner.tsx (inline or separate) ---
// Drop-in replacement for PromoBanner from before
const PromoBanner: React.FC<{
  onPrimary?: () => void;
  imageSrc: string; // <-- required image
  imageAlt?: string;
}> = ({ onPrimary, imageSrc, imageAlt = "AI theming illustration" }) => {
  return (
    <Box
      sx={{
        p: { xs: 2, md: 3 },
        pr: { xs: 2, md: 6 },
        borderRadius: 3,
        border: "1px solid",
        borderColor: "#069668",
        backgroundImage:
          "linear-gradient(90deg, rgba(6,150,104,0.12) 0%, rgba(6,150,104,0.06) 50%, rgba(6,150,104,0.10) 100%)",
        position: "relative",
        overflow: "hidden",
      }}
    >
      <Grid container alignItems="center" spacing={2} wrap="nowrap">
        {/* Left: content */}
        <Grid>
          <Stack spacing={1.5}>
            <Typography fontSize={18} fontWeight={800}>
              Theme your Devportal with AI
            </Typography>
            <Typography fontSize={14} color="text.secondary">
              Generate a polished color palette, typography and layout presets
              for your portal. You can fine-tune everything afterwards.
            </Typography>
            <Box>
              <Button variant="contained" onClick={onPrimary}>
                Start theming
              </Button>
            </Box>
          </Stack>
        </Grid>

        {/* Right: single image */}
        <Grid>
          <Box
            component="img"
            src={imageSrc}
            alt={imageAlt}
            sx={{
              width: 240, // tweak as needed
              height: 140,
              objectFit: "contain",
              display: "block",
              borderRadius: 2,
            }}
          />
        </Grid>
      </Grid>
    </Box>
  );
};

const ThemeSettingsPanel: React.FC = () => {
  const [tab, setTab] = React.useState(0);
  const [published, setPublished] = React.useState(false);

  // colors
  const [bg, setBg] = React.useState("#FFFFFF");
  const [primary, setPrimary] = React.useState("#1A4C6D");
  const [secondary, setSecondary] = React.useState("#FE8C3A");
  const [text, setText] = React.useState("#40404B");

  return (
    <Stack spacing={2.5}>
      {/* Top bar: Org/APIs & switch */}
      <Stack direction="row" alignItems="center" justifyContent="space-between">
        <Tabs
          value={0}
          sx={{ minHeight: 36, "& .MuiTab-root": { minHeight: 36 } }}
        >
          <Tab label="Org Level" />
          <Tab label="API Level" disabled />
        </Tabs>
      </Stack>

      <Typography variant="body2" color="text.secondary">
        Manage Organization Level Themings
      </Typography>

      <Tabs
        value={tab}
        onChange={(_, v) => setTab(v)}
        sx={{ borderBottom: (t) => `1px solid ${t.palette.divider}` }}
      >
        <Tab label="Theme Settings" />
        <Tab label="HTML" />
        <Tab label="CSS" />
        <Tab label="Referenced CSS" />
        <Tab label="Assets" />
      </Tabs>

      {tab === 0 && (
        <Accordion defaultExpanded>
          <AccordionSummary expandIcon={<ExpandMoreIcon />}>
            <Typography>Color Palette</Typography>
          </AccordionSummary>
          <AccordionDetails>
            <Stack spacing={3} sx={{ maxWidth: 560 }}>
              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Background
                </Typography>
                <ColorField
                  label="Main"
                  value={bg}
                  onChange={setBg}
                  width={260}
                />
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Primary
                </Typography>
                <ColorField
                  label="Main"
                  value={primary}
                  onChange={setPrimary}
                  width={260}
                />
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Secondary
                </Typography>
                <ColorField
                  label="Main"
                  value={secondary}
                  onChange={setSecondary}
                  width={260}
                />
              </Box>

              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Text
                </Typography>
                <ColorField
                  label="Main"
                  value={text}
                  onChange={setText}
                  width={260}
                />
              </Box>
            </Stack>
          </AccordionDetails>
        </Accordion>
      )}

      {tab !== 0 && (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography variant="body2" color="text.secondary">
            This tab is a placeholder. Hook up your real editors here.
          </Typography>
        </Paper>
      )}
    </Stack>
  );
};

/** ------- PAGE ------- */

const PortalManagement: React.FC = () => {
  const [portalType, setPortalType] = React.useState<PortalType>("private");
  const [portalName, setPortalName] = React.useState<string>("");
  const [step, setStep] = React.useState<Step>("setup");

  return (
    <Box sx={{ p: 3, overflowX: "auto" }}>
      <Typography variant="h5" sx={{ mt: 0.5 }}>
        {step === "setup" ? "Create portal" : "Theme Settings"}
      </Typography>
      <Typography
        variant="body2"
        color="text.secondary"
        sx={{ mt: 0.5, mb: 3, maxWidth: 760 }}
      >
        {step === "setup"
          ? "Define visibility of your portal and publish your first API. You can modify your selections later."
          : "Manage and customize the theme settings for your organization."}
      </Typography>

      <Grid
        container
        columnSpacing={3}
        alignItems="flex-start"
        sx={{ flexWrap: "nowrap", minWidth: 960 }}
      >
        {/* Left column */}
        <Grid>
          {step === "setup" ? (
            <Stack spacing={2.5}>
              <Typography variant="subtitle2">
                Choose Your Portal Type
              </Typography>
              <Grid container spacing={2}>
                <Grid>
                  <OptionCard
                    title="Internal portal"
                    description="Only authenticated users can view pages and APIs."
                    icon={<LockOutlinedIcon />}
                    selected={portalType === "private"}
                    onClick={() => setPortalType("private")}
                  />
                </Grid>
                <Grid>
                  <OptionCard
                    title="Developer portal"
                    description="Anyone with the link can view portal pages and APIs."
                    icon={<PublicOutlinedIcon />}
                    selected={portalType === "public"}
                    onClick={() => setPortalType("public")}
                  />
                </Grid>
              </Grid>

              <Box>
                <Typography variant="subtitle2" sx={{ mb: 1 }}>
                  Add Your Portal Details
                </Typography>
                <FormControl fullWidth sx={{ mt: 1 }}>
                  <TextField
                    label="Portal name"
                    placeholder="Provide a name for your portal"
                    value={portalName}
                    onChange={(e) => setPortalName(e.target.value)}
                    required
                    sx={{ maxWidth: 580 }}
                  />
                </FormControl>
              </Box>

              <Stack direction="row" gap={2}>
                <Button
                  variant="contained"
                  disabled={!portalName.trim()}
                  onClick={() => setStep("theme")}
                >
                  Create and continue
                </Button>
                <Button variant="text" onClick={() => setStep("theme")}>
                  Skip for now
                </Button>
              </Stack>
            </Stack>
          ) : (
            <Stack spacing={2.5}>
              <PromoBanner
                imageSrc={AiBanner}
                onPrimary={() => {
                  /* open AI flow */
                }}
              />
              <ThemeSettingsPanel />
            </Stack>
          )}
        </Grid>

        {/* Right column (preview) */}
        <Grid>
          <Box sx={{ position: "sticky", top: 24 }}>
            {portalType === "private" ? <PrivatePreview /> : <PublicPreview />}
          </Box>
        </Grid>
      </Grid>
    </Box>
  );
};

export default PortalManagement;
