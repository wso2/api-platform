import React from "react";
import {
  Box,
  Typography,
  Tooltip,
  CardContent,
  Stack,
  Chip,
} from "@mui/material";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";
import apisData from "../../data/apis.json";
import { IconButton } from "../../components/src/components/IconButton";
import Copy from "../../components/src/Icons/generated/Copy";
import { Card } from "../../components/src/components/Card";
import theme from "../../theme";

// local helpers (kept inside this file)
type ApiItem = {
  id: string;
  name: string;
  owner: string;
  version: string;
  context: string;
  description: string;
  tags: string[];
  extraTagsCount: number;
  rating: number;
  disabled?: boolean;
};

function twoLetters(s: string) {
  const letters = (s || "").replace(/[^A-Za-z]/g, "");
  if (!letters) return "API";
  return (letters[0] + (letters[1] || "")).toUpperCase();
}

function testCurl(api: ApiItem) {
  // simple local test endpoint based on context/version
  return `curl -sS -X GET "http://localhost:8080${api.context}/v${api.version}/health" -H "accept: application/json"`;
}

// deterministic tiny generator for a sparkline
function makeSeries(seedStr: string, n = 24) {
  let seed = 0;
  for (let i = 0; i < seedStr.length; i++)
    seed = (seed * 31 + seedStr.charCodeAt(i)) >>> 0;
  const series: number[] = [];
  let v = (seed % 50) + 40; // base
  for (let i = 0; i < n; i++) {
    // pseudo variation
    seed = (1664525 * seed + 1013904223) >>> 0;
    const jitter = (seed % 15) - 7; // -7..+7
    v = Math.max(10, v + jitter);
    series.push(v);
  }
  return series;
}

function MiniLineChart({
  data,
  width = 640,
  height = 180,
}: {
  data: number[];
  width?: number;
  height?: number;
}) {
  const pad = 16;
  const w = width - pad * 2;
  const h = height - pad * 2;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const xStep = w / Math.max(1, data.length - 1);
  const scaleY = (v: number) => h - ((v - min) / Math.max(1, max - min)) * h;

  const d = data
    .map(
      (v, i) => `${i === 0 ? "M" : "L"} ${pad + i * xStep} ${pad + scaleY(v)}`
    )
    .join(" ");

  return (
    <Box sx={{ width, height }}>
      <svg
        width={width}
        height={height}
        role="img"
        aria-label="API traffic line chart"
      >
        {/* grid */}
        <line
          x1={pad}
          y1={pad}
          x2={pad}
          y2={height - pad}
          stroke="#e0e0e0"
          strokeWidth="1"
        />
        <line
          x1={pad}
          y1={height - pad}
          x2={width - pad}
          y2={height - pad}
          stroke="#e0e0e0"
          strokeWidth="1"
        />
        {/* path */}
        <path d={d} fill="none" stroke="#1976d2" strokeWidth="2.25" />
      </svg>
    </Box>
  );
}

export default function StepThreeTest({
  notify,
}: {
  notify: (msg: string) => void; // snackbar from parent
}) {
  const arr = apisData as ApiItem[];
  const api = arr?.[0];

  const [showGraph, setShowGraph] = React.useState(false);

  if (!api) {
    return (
      <Box textAlign="center" py={6}>
        <Typography variant="h6" fontWeight={800} gutterBottom>
          Test Your APIs
        </Typography>
        <Typography color="text.secondary">No APIs found in data.</Typography>
      </Box>
    );
  }

  const cmd = testCurl(api);

  const copyCmd = async () => {
    try {
      await navigator.clipboard.writeText(cmd);
      setShowGraph(true);
      notify("Test curl copied. Showing analytics.");
    } catch {
      notify("Unable to copy command");
    }
  };

  // analytics derived from the mock series
  const series = React.useMemo(() => makeSeries(api.name, 24), [api.name]);
  const total = series.reduce((a, b) => a + b, 0);
  const last = series[series.length - 1];
  const max = Math.max(...series);
  const successRate = 99 - (api.name.length % 3); // playful, stable number

  return (
    <Box py={2}>
      <Typography variant="h6" fontWeight={600}>
        Test Your APIs
      </Typography>
      <Typography fontSize={13} color="text.secondary">
        Run the test command locally, then review live analytics.
      </Typography>

      {/* Curl block */}
      <Box sx={{ mt: 2, position: "relative" }}>
        <Box
          sx={{
            bgcolor: "#373842",
            color: "#EDEDF0",
            p: 2.5,
            borderRadius: 1,
            fontFamily:
              'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
            fontSize: 14,
            lineHeight: 1.6,
            overflowX: "auto",
          }}
        >
          {cmd}
        </Box>

        <Tooltip title="Copy command">
          <IconButton
            onClick={copyCmd}
            sx={{
              position: "absolute",
              top: 10,
              right: 6,
            }}
          >
            <Copy />
          </IconButton>
        </Tooltip>
      </Box>

      {/* Analytics after copy */}
      {showGraph && (
        <Card style={{ marginTop: 16, padding: 16 }} testId={""}>
          <CardContent sx={{ p: 0 }}>
            <Stack
              direction="row"
              alignItems="center"
              spacing={2}
              sx={{ mb: 2 }}
            >
              <Box
                sx={{
                  width: 85,
                  height: 65,
                  borderRadius: 1,
                  backgroundImage: `linear-gradient(135deg,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).light} 0%,
    #059669 55%,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).dark} 100%)`,
                  color: "common.white",
                  fontWeight: 800,
                  fontSize: 20,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                {twoLetters(api.name)}
              </Box>

              <Box sx={{ flex: 1 }}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="h6" sx={{ lineHeight: 1.2 }}>
                    {api.name}
                  </Typography>
                  <Chip
                    size="small"
                    label={api.tags?.[0] || "General"}
                    color="info"
                    variant="outlined"
                    sx={{ borderRadius: 1 }}
                  />
                </Stack>
                <Typography
                  fontSize={12}
                  color="text.secondary"
                  sx={{ mt: 0.5 }}
                >
                  {api.description || "No description"}
                </Typography>
              </Box>

              {/* Quick stats */}
              <Stack direction="row" spacing={2}>
                <Chip label={`Total: ${total}`} variant="outlined" />
                <Chip label={`Peak: ${max}`} variant="outlined" />
                <Chip label={`Last: ${last}`} variant="outlined" />
                <Chip
                  label={`Success: ${successRate}%`}
                  color="success"
                  variant="outlined"
                />
              </Stack>
            </Stack>

            <MiniLineChart data={series} />
          </CardContent>
        </Card>
      )}
    </Box>
  );
}
