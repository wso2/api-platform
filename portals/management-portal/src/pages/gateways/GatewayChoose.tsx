import React from "react";
import { Box, Grid, Tabs, Tab, Typography, Chip } from "@mui/material";
import OpenInNewOutlinedIcon from "@mui/icons-material/OpenInNewOutlined";
import ArrowForwardIosIcon from "@mui/icons-material/ArrowForwardIos";
import MenuBookOutlinedIcon from "@mui/icons-material/MenuBookOutlined";
import ShieldOutlinedIcon from "@mui/icons-material/ShieldOutlined";
import BuildOutlinedIcon from "@mui/icons-material/BuildOutlined";

import {
  Card,
  CardActionArea,
  CardContent,
} from "../../components/src/components/Card";

import hybridImg from "../../images/hybrid-gateway.svg";
import cloudImg from "../../images/cloud-gateway.svg";
import type { GatewayType } from "../../hooks/gateways";

type Props = {
  onSelectType: (t: GatewayType) => void;
};

const UseCaseCard: React.FC<{
  title: string;
  desc: string;
  onClick?: () => void;
  external?: boolean;
  leftIcon?: React.ReactNode;
  /** Visual tone for the small left icon box */
  tone?: "blue" | "green" | "purple" | "gray";
}> = ({ title, desc, onClick, external, leftIcon, tone = "gray" }) => {
  const toneMap = {
    blue: { bg: "rgba(13,153,255,0.10)", border: "#0d99ff", icon: "#0d99ff" },
    green: { bg: "rgba(6,150,104,0.12)", border: "#069668", icon: "#069668" },
    purple: { bg: "rgba(136,84,208,0.12)", border: "#8854d0", icon: "#8854d0" },
    gray: { bg: "rgba(0,0,0,0.06)", border: "#d0d5dd", icon: "#667085" },
  } as const;

  const t = toneMap[tone];

  return (
    <Card testId={""}>
      <CardActionArea onClick={onClick} testId={""}>
        <CardContent
          sx={{
            minHeight: 112,
            display: "flex",
            alignItems: "flex-start",
            justifyContent: "space-between",
            gap: 2,
            maxWidth: 360,
          }}
        >
          <Box sx={{ display: "flex", gap: 2 }}>
            <Box
              sx={{
                width: 28,
                height: 28,
                borderRadius: 1,
                display: "flex",
                alignItems: "center",
                justifyContent: "center",
                mt: 0.5,
                backgroundColor: t.bg,
                border: `1px solid ${t.border}`,
                color: t.icon,
                padding:2.5
              }}
            >
              {leftIcon ?? <MenuBookOutlinedIcon fontSize="small" />}
            </Box>
            <Box>
              <Typography fontWeight={600}>{title}</Typography>
              <Typography color="text.secondary" variant="body2">
                {desc}
              </Typography>
            </Box>
          </Box>

          <Box sx={{ mt: 0.5 }}>
            {external ? (
              <OpenInNewOutlinedIcon fontSize="small" />
            ) : (
              <ArrowForwardIosIcon fontSize="small" />
            )}
          </Box>
        </CardContent>
      </CardActionArea>
    </Card>
  );
};

const GatewayChoose: React.FC<Props> = ({ onSelectType }) => {
  const [selectedType, setSelectedType] = React.useState<GatewayType>("hybrid");
  const [useCaseTab, setUseCaseTab] = React.useState<"api" | "ai">("api");

  const handlePick = (t: GatewayType) => {
    setSelectedType(t);
    onSelectType(t);
  };

  // Inline style for custom Card (no sx supported on your Card)
  const cardStyle = (selected: boolean): React.CSSProperties => ({
    position: "relative",
    borderRadius: 12,
    borderWidth: selected ? 1 : 1,
    borderStyle: "solid",
    borderColor: selected ? "#069668" : "#e0e0e0",
    boxShadow: selected ? "0 0 0 2px rgba(6,150,104,0.15)" : "none",
    transition:
      "box-shadow 0.2s ease, border-color 0.2s ease, transform 0.1s ease",
    transform: selected ? "translateY(-1px)" : "none",
  });

  return (
    <>
      <Box mb={3}>
        <Typography variant="h3" fontWeight={700}>
          Add Your Gateway
        </Typography>

        <Typography variant="body2">
          Let’s get started with Managing your API Proxies/MCP & API Product
        </Typography>
      </Box>

      <Grid container spacing={4}>
        {/* Hybrid */}
        <Grid>
          <Card testId={""} style={cardStyle(selectedType === "hybrid")}>
            <CardActionArea
              sx={{ height: "100%" }}
              onClick={() => handlePick("hybrid")}
              testId={""}
              aria-pressed={selectedType === "hybrid"}
            >
              <CardContent
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 2,
                }}
              >
                <Box padding={2}>
                  <Box
                    component="img"
                    src={hybridImg}
                    alt="Hybrid Gateway"
                    sx={{ width: 140 }}
                  />
                </Box>
                <Box display="flex" alignItems="center" flexDirection="column">
                  <Typography fontSize={18} fontWeight={600}>
                    On Premise Gateway
                  </Typography>
                  <Typography align="center">
                    Let’s get started with creating your Gateways
                  </Typography>
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        </Grid>

        {/* Cloud */}
        <Grid>
          <Card testId={""} style={cardStyle(selectedType === "cloud")}>
            <CardActionArea
              sx={{ height: "100%" }}
              onClick={() => handlePick("cloud")}
              testId={""}
              aria-pressed={selectedType === "cloud"}
            >
              <CardContent
                sx={{
                  height: "100%",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  justifyContent: "center",
                  gap: 2,
                }}
              >
                <Box padding={2}>
                  <Box
                    component="img"
                    src={cloudImg}
                    alt="Cloud Gateway"
                    sx={{ width: 132 }}
                  />
                </Box>
                <Box display="flex" alignItems="center" flexDirection="column">
                  <Typography fontSize={18} fontWeight={600}>
                    Cloud Gateway
                  </Typography>
                  <Typography align="center">
                    Let’s get started with creating your Gateways
                  </Typography>
                </Box>
              </CardContent>
            </CardActionArea>
          </Card>
        </Grid>
      </Grid>

      {/* Use cases */}
      <Box mt={6} display={"flex"} flexDirection={"column"}>
        <Box>
          <Typography variant="body1" fontWeight={600} sx={{ mb: 1 }}>
            Start with a popular use case
          </Typography>

          <Tabs
            value={useCaseTab}
            onChange={(_, v: "api" | "ai") => setUseCaseTab(v)}
            sx={{
              mb: 2,
              "& .MuiTab-root": { textTransform: "none", minHeight: 36 },
            }}
          >
            <Tab label="API Gateway" value="api" />
            <Tab label="AI Gateway" value="ai" />
          </Tabs>
        </Box>

        {useCaseTab === "api" ? (
          <Grid container spacing={2}>
            <Grid>
              <UseCaseCard
                title="Prepare a public API for launch"
                desc="Add authentication and rate limits before launching."
                leftIcon={<BuildOutlinedIcon fontSize="small" />}
                tone="green"
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Protect your APIs with OpenID Connect"
                desc="Authorize API clients using your IdP."
                leftIcon={<ShieldOutlinedIcon fontSize="small" />}
                tone="purple"
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Explore detailed docs"
                desc="Dive deeper into guides and best practices on our developer site."
                external
                onClick={() =>
                  window.open("https://example.dev/docs", "_blank")
                }
                tone="blue"
              />
            </Grid>
          </Grid>
        ) : (
          <Grid container spacing={2}>
            <Grid>
              <UseCaseCard
                title="Getting started guide"
                desc="Test your AI app with built-in security, metrics, and monitoring."
                external
                onClick={() =>
                  window.open("https://example.dev/ai/get-started", "_blank")
                }
                tone="blue"
              />
            </Grid>
            <Grid>
              <UseCaseCard
                title="Explore detailed docs"
                desc="Dive deeper into guides and best practices on our developer site."
                external
                onClick={() =>
                  window.open("https://example.dev/ai/docs", "_blank")
                }
                tone="blue"
              />
            </Grid>
          </Grid>
        )}
      </Box>
    </>
  );
};

export default GatewayChoose;
