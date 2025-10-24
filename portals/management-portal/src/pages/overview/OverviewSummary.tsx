import React from "react";
import {
  Box,
  Typography,
  Stack,
  TextField,
  InputAdornment,
  Chip,
  CardContent,
  Divider,
  TableContainer,
  Table,
  TableHead,
  TableRow,
  TableCell,
  TableBody,
  Avatar,
  Paper,
} from "@mui/material";
import SearchIcon from "@mui/icons-material/Search";
import AddIcon from "@mui/icons-material/Add";
import AccessTimeIcon from "@mui/icons-material/AccessTime";

import apisData from "../../data/apis.json";
import type { GatewayRecord } from "./types";
import { IconButton } from "../../components/src/components/IconButton";
import { Button } from "../../components/src/components/Button";
// If you still use it in other places you can keep it,
// otherwise you may remove this import.
// import { SearchBar } from "../../components/src/components/SearchBar/SearchBar";
import { Card } from "../../components/src/components/Card";
import theme from "../../theme";
import TrendingUpIcon from "@mui/icons-material/TrendingUp";
import ErrorOutlineIcon from "@mui/icons-material/ErrorOutline";
import StackedLineChartIcon from "@mui/icons-material/StackedLineChart";
import ChevronRightRoundedIcon from "@mui/icons-material/ChevronRightRounded";


/* ------------------ helpers ------------------ */

const twoLetters = (s: string) => {
  const letters = (s || "").replace(/[^A-Za-z]/g, "");
  if (!letters) return "GW";
  return (letters[0] + (letters[1] || "")).toUpperCase();
};

const relativeTime = (d: Date) => {
  const diff = Math.max(0, Date.now() - new Date(d).getTime());
  const sec = Math.floor(diff / 1000);
  const min = Math.floor(sec / 60);
  const hr = Math.floor(min / 60);
  const day = Math.floor(hr / 24);
  if (sec < 45) return "just now";
  if (min < 60) return `${min} min ago`;
  if (hr < 24) return `${hr} hr${hr > 1 ? "s" : ""} ago`;
  return `${day} day${day > 1 ? "s" : ""} ago`;
};

// Deterministic “fake” updatedAt for APIs (if your JSON lacks dates)
const seededUpdatedAt = (seedStr: string) => {
  let seed = 0;
  for (let i = 0; i < seedStr.length; i++)
    seed = (seed * 31 + seedStr.charCodeAt(i)) >>> 0;
  // 7–60 days ago
  const daysAgo = 7 + (seed % 54);
  const d = new Date();
  d.setDate(d.getDate() - daysAgo);
  return d;
};

// very small inline chart (no deps)
function makeSeries(seedStr: string, n = 24) {
  let seed = 0;
  for (let i = 0; i < seedStr.length; i++)
    seed = (seed * 31 + seedStr.charCodeAt(i)) >>> 0;
  const series: number[] = [];
  let v = (seed % 50) + 40;
  for (let i = 0; i < n; i++) {
    seed = (1664525 * seed + 1013904223) >>> 0;
    const jitter = (seed % 15) - 7;
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
      <svg width={width} height={height} role="img" aria-label="API analytics">
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
        <path d={d} fill="none" stroke="#1976d2" strokeWidth="2.25" />
      </svg>
    </Box>
  );
}

/* ------------------ types ------------------ */

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
  // Optional: If your JSON has updatedAt, you can add it here:
  // updatedAt?: string;
};

/* ------------------ component ------------------ */

export default function OverviewSummary({
  gateways,
  navigateToGateways,
  navigateToApis,
}: {
  gateways: GatewayRecord[];
  navigateToGateways: () => void;
  navigateToApis: () => void;
}) {
  /* ---- Gateways search ---- */
  const [gq, setGq] = React.useState("");
  const gwList = React.useMemo(() => {
    const q = gq.trim().toLowerCase();
    if (!q) return gateways;
    return gateways.filter(
      (g) =>
        g.displayName.toLowerCase().includes(q) ||
        g.name.toLowerCase().includes(q) ||
        (g.host || "").toLowerCase().includes(q)
    );
  }, [gq, gateways]);

  /* ---- APIs search ---- */
  const [aq, setAq] = React.useState("");
  const apiList = React.useMemo(() => {
    const arr = apisData as ApiItem[];
    const q = aq.trim().toLowerCase();
    if (!q) return arr;
    return arr.filter(
      (a) =>
        a.name.toLowerCase().includes(q) ||
        a.context.toLowerCase().includes(q) ||
        a.tags.some((t) => t.toLowerCase().includes(q))
    );
  }, [aq]);

  return (
    <Box>
      {/* -------- Gateways header -------- */}
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ mb: 0.5}}
      >
        <Typography variant="h6">Gateways</Typography>
        <Stack direction="row" spacing={1} alignItems="center">
          <TextField
            size="small"
            placeholder="Search gateways"
            value={gq}
            onChange={(e) => setGq(e.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <IconButton edge="start" disableRipple tabIndex={-1}>
                    <SearchIcon fontSize="small" />
                  </IconButton>
                </InputAdornment>
              ),
            }}
          />
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={navigateToGateways}
          >
            Add Gateway
          </Button>
        </Stack>
      </Stack>

      {/* -------- Gateways table -------- */}
      <TableContainer
        component={Paper}
        elevation={0}
        sx={{ mb: 3, borderRadius: 3 }}
      >
        <Table
          sx={{
            borderCollapse: "separate",
            borderSpacing: "0 12px", // vertical gap between rows
          }}
        >
          <TableHead
            sx={{
              "& th": {
                borderBottom: "none", // <-- remove header underline
                bgcolor: "transparent",
              },
            }}
          >
            <TableRow>
              <TableCell sx={{ width: "35%" }}>
                <Typography variant="caption" color="text.secondary">
                  Name
                </Typography>
              </TableCell>
              <TableCell sx={{ width: "35%" }}>
                <Typography variant="caption" color="text.secondary">
                  Description
                </Typography>
              </TableCell>
              <TableCell sx={{ width: 190 }}>
                <Typography variant="caption" color="text.secondary">
                  Type
                </Typography>
              </TableCell>
              <TableCell sx={{ width: 180 }}>
                <Typography variant="caption" color="text.secondary">
                  Last Updated
                </Typography>
              </TableCell>
            </TableRow>
          </TableHead>

          <TableBody>
            {gwList.map((g) => (
              <TableRow
                key={g.id}
                sx={{
                  // make the whole row look like one card
                  "& td": {
                    bgcolor: "#f8f8f8ff",
                    // border: "1px solid",
                    // borderColor: "divider",
                    py: 1.5,
                    // soft shadow
                    boxShadow:
                      "0 1px 1px rgba(59, 62, 67, 0.06), 0 1px 1px rgba(112, 114, 118, 0.04)",
                  },
                  // remove inner vertical seams so border looks continuous
                  "& td + td": {
                    borderLeft: "none",
                  },
                  // rounded ends
                  "& td:first-of-type": {
                    borderTopLeftRadius: 8,
                    borderBottomLeftRadius: 8,
                  },
                  "& td:last-of-type": {
                    borderTopRightRadius: 8,
                    borderBottomRightRadius: 8,
                  },
                  // hover polish
                  "&:hover td": {
                    bgcolor: "grey.50",
                  },
                }}
              >
                {/* Name */}
                <TableCell>
                  <Stack
                    direction="row"
                    spacing={2}
                    alignItems="center"
                    minWidth={0}
                  >
                    <Avatar
                      sx={{
                        width: 40,
                        height: 40,
                        fontWeight: 700,
                        borderRadius: 1,
                        backgroundImage: `linear-gradient(135deg,
                          ${
                            theme.palette.augmentColor({
                              color: { main: "#059669" },
                            }).light
                          } 0%,
                          #059669 55%,
                          ${
                            theme.palette.augmentColor({
                              color: { main: "#059669" },
                            }).dark
                          } 100%)`,
                        color: "common.white",
                      }}
                      variant="rounded"
                    >
                      {twoLetters(g.displayName || g.name).slice(0, 1)}
                    </Avatar>
                    <Typography
                      fontSize={14}
                      noWrap
                      title={g.displayName || g.name}
                    >
                      {g.displayName || g.name}
                    </Typography>
                    {g.isActive && (
                      <Chip
                        size="small"
                        label="Active"
                        color="success"
                        sx={{ borderRadius: 1, ml: 1 }}
                      />
                    )}
                  </Stack>
                </TableCell>

                {/* Description */}
                <TableCell sx={{ maxWidth: 520 }}>
                  <Typography
                    variant="body2"
                    noWrap
                    title={g.description || "—"}
                  >
                    {g.description || "—"}
                  </Typography>
                </TableCell>

                {/* Type (match screenshot style; keep simple “HTTP”) */}
                <TableCell>
                  <Typography fontSize={12}>Cloud Gateway</Typography>
                </TableCell>

                {/* Last Updated (using createdAt here; replace with your own updatedAt if available) */}
                <TableCell>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <AccessTimeIcon
                      fontSize="small"
                      sx={{ color: "text.disabled" }}
                    />
                    <Typography fontSize={12} color="text.primary">
                      {relativeTime(g.createdAt)}
                    </Typography>
                  </Stack>
                </TableCell>
              </TableRow>
            ))}

            {gwList.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Card variant="outlined" testId={""}>
                    <Typography color="text.secondary">
                      No gateways match your search.
                    </Typography>
                  </Card>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* -------- APIs header -------- */}
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ mb: 0.5, mt: 2 }}
      >
        <Typography variant="h6" fontWeight={800}>
          APIs
        </Typography>
        <Stack direction="row" spacing={1} alignItems="center">
          <TextField
            size="small"
            placeholder="Search APIs"
            value={aq}
            onChange={(e) => setAq(e.target.value)}
            InputProps={{
              startAdornment: (
                <InputAdornment position="start">
                  <IconButton edge="start" disableRipple tabIndex={-1}>
                    <SearchIcon fontSize="small" />
                  </IconButton>
                </InputAdornment>
              ),
            }}
          />
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={navigateToApis}
            sx={{ textTransform: "none" }}
          >
            Add
          </Button>
        </Stack>
      </Stack>

      {/* -------- APIs table -------- */}
      <TableContainer component={Paper} elevation={0} sx={{ mb: 3 }}>
        <Table
          sx={{
            borderCollapse: "separate",
            borderSpacing: "0 12px", // vertical gap between rows
          }}
        >
          <TableHead
            sx={{
              "& th": {
                borderBottom: "none", // <-- remove header underline
                bgcolor: "transparent",
              },
            }}
          >
            <TableRow>
              <TableCell sx={{ width: "30%" }}>
                <Typography variant="caption" color="text.secondary">
                  Name
                </Typography>
              </TableCell>
              <TableCell sx={{ width: "50%" }}>
                <Typography variant="caption" color="text.secondary">
                  Description
                </Typography>
              </TableCell>
              <TableCell sx={{ width: 140 }}>
                <Typography variant="caption" color="text.secondary">
                  Type
                </Typography>
              </TableCell>
              <TableCell sx={{ width: 400 }}>
                <Typography variant="caption" color="text.secondary">
                  Last Updated
                </Typography>
              </TableCell>
            </TableRow>
          </TableHead>

          <TableBody>
            {(apiList as ApiItem[]).map((api) => {
              // If your JSON has an `updatedAt`, use that; else seeded date
              const updatedAt =
                /* api.updatedAt ? new Date(api.updatedAt) : */ seededUpdatedAt(
                  api.id || api.name
                );
              return (
                <TableRow
                  key={api.id}
                  hover
                  sx={{
                    // make the whole row look like one card
                    "& td": {
                      bgcolor: "#f8f8f8ff",
                      // border: "1px solid",
                      // borderColor: "divider",
                      py: 1.5,
                      // soft shadow
                      boxShadow:
                        "0 1px 1px rgba(59, 62, 67, 0.06), 0 1px 1px rgba(112, 114, 118, 0.04)",
                    },
                    // remove inner vertical seams so border looks continuous
                    "& td + td": {
                      borderLeft: "none",
                    },
                    // rounded ends
                    "& td:first-of-type": {
                      borderTopLeftRadius: 8,
                      borderBottomLeftRadius: 8,
                    },
                    "& td:last-of-type": {
                      borderTopRightRadius: 8,
                      borderBottomRightRadius: 8,
                    },
                    // hover polish
                    "&:hover td": {
                      bgcolor: "grey.50",
                    },
                  }}
                >
                  {/* Name */}
                  <TableCell>
                    <Stack
                      direction="row"
                      spacing={2}
                      alignItems="center"
                      minWidth={0}
                    >
                      <Avatar
                        variant="rounded"
                        sx={{
                          width: 40,
                          height: 40,
                          fontWeight: 700,
                          borderRadius: 1,
                          backgroundImage: `linear-gradient(135deg,
                          ${
                            theme.palette.augmentColor({
                              color: { main: "#059669" },
                            }).light
                          } 0%,
                          #059669 55%,
                          ${
                            theme.palette.augmentColor({
                              color: { main: "#059669" },
                            }).dark
                          } 100%)`,
                          color: "common.white",
                        }}
                      >
                        {twoLetters(api.name).slice(0, 1)}
                      </Avatar>
                      <Typography fontSize={14} noWrap title={api.name}>
                        {api.name}
                      </Typography>
                    </Stack>
                  </TableCell>

                  {/* Description */}
                  <TableCell sx={{ maxWidth: 520 }}>
                    <Typography fontSize={12} noWrap title={api.description}>
                      {api.description}
                    </Typography>
                  </TableCell>

                  {/* Type (to match screenshot, keep “HTTP”) */}
                  <TableCell>
                    <Typography fontSize={12}>HTTP</Typography>
                  </TableCell>

                  {/* Last Updated */}
                  <TableCell>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <AccessTimeIcon
                        fontSize="small"
                        sx={{ color: "text.disabled" }}
                      />
                      <Typography fontSize={12} color="text.primary">
                        {relativeTime(updatedAt)}
                      </Typography>
                    </Stack>
                  </TableCell>
                </TableRow>
              );
            })}

            {apiList.length === 0 && (
              <TableRow>
                <TableCell colSpan={4}>
                  <Card variant="outlined" testId={""}>
                    <Typography color="text.secondary">
                      No APIs match your search.
                    </Typography>
                  </Card>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>

      {/* -------- API Insights (unchanged) -------- */}
      <Typography variant="h6" fontWeight={800} sx={{ mb: 1 }}>
        Insights{" "}
        <Typography component="span" variant="subtitle2" color="text.secondary">
          (Last 24 hours)
        </Typography>
      </Typography>

      {(() => {
        const arr = apisData as ApiItem[];
        const api = arr?.[0];

        // fallback cards when no data
        if (!api) {
          return (
            <Box
              sx={{
                display: "grid",
                gridTemplateColumns: { xs: "1fr", md: "repeat(4, 1fr)" },
                gap: 2,
              }}
            >
              {[
                "Requests",
                "Errors",
                "Average TPS",
                "Latency (95th Percentile)",
              ].map((label) => (
                <Box
                  key={label}
                  sx={{
                    p: 2.5,
                    border: "1px solid",
                    borderColor: "divider",
                    borderRadius: 3,
                    bgcolor: "background.paper",
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                  }}
                >
                  <Stack direction="row" spacing={2} alignItems="center">
                    <Box
                      sx={{
                        width: 40,
                        height: 40,
                        borderRadius: "50%",
                        bgcolor: "action.hover",
                      }}
                    />
                    <Box>
                      <Typography variant="h5" fontWeight={800}>
                        0
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {label}
                      </Typography>
                    </Box>
                  </Stack>
                  <Box
                    sx={{
                      width: 28,
                      height: 28,
                      borderRadius: "50%",
                      border: "1px solid",
                      borderColor: "divider",
                      display: "grid",
                      placeItems: "center",
                      color: "text.secondary",
                    }}
                  >
                    <ChevronRightRoundedIcon fontSize="small" />
                  </Box>
                </Box>
              ))}
            </Box>
          );
        }

        // compute metrics from the demo series
        const series = makeSeries(api.name, 24); // 24 hourly buckets
        const total = series.reduce((a, b) => a + b, 0);
        const avgTps = Math.round(total / (24 * 60 * 60)); // naive demo calc
        const errorsPct = 0; // demo placeholder
        const p95LatencyMs = 0; // demo placeholder

        const cards = [
          {
            id: "req",
            icon: <TrendingUpIcon fontSize="small" />,
            value: total,
            label: "Requests",
            sub: "",
          },
          {
            id: "err",
            icon: <ErrorOutlineIcon fontSize="small" />,
            value: `${errorsPct} %`,
            label: "Errors",
            sub: "",
          },
          {
            id: "tps",
            icon: <StackedLineChartIcon fontSize="small" />,
            value: avgTps,
            label: "Average TPS",
            sub: "",
          },
          {
            id: "lat",
            icon: <AccessTimeIcon fontSize="small" />,
            value: `${p95LatencyMs} ms`,
            label: "Latency",
            sub: "(95th Percentile)",
          },
        ];

        return (
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr", md: "repeat(4, 1fr)" },
              gap: 2,
            }}
          >
            {cards.map((c) => (
              <Box
                key={c.id}
                sx={{
                  p: 2.5,
                  border: "1px solid",
                  borderColor: "divider",
                  borderRadius: 3,
                  bgcolor: "background.paper",
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "space-between",
                }}
              >
                <Stack direction="row" spacing={2} alignItems="center">
                  <Box
                    sx={{
                      width: 40,
                      height: 40,
                      borderRadius: "50%",
                      bgcolor: "action.hover",
                      display: "grid",
                      placeItems: "center",
                      color: "primary.main",
                    }}
                  >
                    {c.icon}
                  </Box>
                  <Box>
                    <Typography variant="h5" fontWeight={800}>
                      {c.value}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {c.label}
                      {c.sub ? (
                        <>
                          <br />
                          {c.sub}
                        </>
                      ) : null}
                    </Typography>
                  </Box>
                </Stack>

                <Box
                  sx={{
                    width: 28,
                    height: 28,
                    borderRadius: "50%",
                    border: "1px solid",
                    borderColor: "divider",
                    display: "grid",
                    placeItems: "center",
                    color: "text.secondary",
                  }}
                >
                  <ChevronRightRoundedIcon fontSize="small" />
                </Box>
              </Box>
            ))}
          </Box>
        );
      })()}
    </Box>
  );
}
