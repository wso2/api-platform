import * as React from "react";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Paper,
  Stack,
  Tab,
  Tabs,
  TextField,
  Typography,
} from "@mui/material";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";

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
      endAdornment: <Swatch value={value} />,
    }}
  />
);

const ThemeSettingsPanel: React.FC = () => {
  const [tab, setTab] = React.useState(0);
  const [bg, setBg] = React.useState("#FFFFFF");
  const [primary, setPrimary] = React.useState("#1A4C6D");
  const [secondary, setSecondary] = React.useState("#FE8C3A");
  const [text, setText] = React.useState("#40404B");

  return (
    <Stack spacing={2.5}>
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

      {tab === 0 ? (
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
      ) : (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Typography variant="body2" color="text.secondary">
            This tab is a placeholder. Hook up your real editors here.
          </Typography>
        </Paper>
      )}
    </Stack>
  );
};

export default ThemeSettingsPanel;
