// src/pages/.../StepTwoApis.tsx
import React from "react";
import { Box, Typography, Tooltip, Stack, Chip } from "@mui/material";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";

import apisData from "../../data/apis.json";
import { twoLetters } from "./utils";
import type { GatewayRecord } from "./types";
import ScienceOutlinedIcon from "@mui/icons-material/ScienceOutlined";
import { Button } from "../../components/src/components/Button";
import { IconButton } from "../../components/src/components/IconButton";
import { Card } from "../../components/src/components/Card";
import CardContent from "@mui/material/CardContent";
import theme from "../../theme";

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

export default function StepTwoApis({
  gateways,
  onGoStep1,
  onGatewayActivated,
  notify,
  onGoStep3,
}: {
  gateways: GatewayRecord[];
  onGoStep1: () => void;
  onGatewayActivated: (id: string) => void; // mark gateway active
  notify: (msg: string) => void; // snackbar
  onGoStep3: () => void; // go to third step
}) {
  const [showApiDetails, setShowApiDetails] = React.useState(false);

  // Copy command, mark active in parent, reveal details
  const copyCurl = async (g: GatewayRecord) => {
    try {
      await navigator.clipboard.writeText(
        "api-platform gateway push --file https://bijira.dev/samples/api.yaml"
      );
      onGatewayActivated(g.id);
      setShowApiDetails(true);
      notify("Command copied. Gateway marked Active.");
    } catch {
      notify("Unable to copy command");
    }
  };

  if (gateways.length === 0) {
    return (
      <Box textAlign="center">
        <Typography variant="h6" mb={1} fontWeight={600} gutterBottom>
          Add Your APIs
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 2 }}>
          You need at least one Gateway before adding APIs.
        </Typography>
        <Button
          variant="contained"
          onClick={onGoStep1}
          sx={{
            textTransform: "none",
          }}
        >
          Go to Step 1
        </Button>
      </Box>
    );
  }

  // Use the newest gateway (index 0)
  const g = gateways[0];

  return (
    <Box py={1}>
      <Typography variant="h6" fontWeight={600}>
        Add Your APIs
      </Typography>

      <Box>
        <Typography variant="body2" sx={{ mb: 1 }}>
          Run this command locally
        </Typography>

        <Box sx={{ position: "relative" }}>
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
            <Box component="span" sx={{ color: "#C5E478", fontWeight: 700 }}>
              api-platform
            </Box>{" "}
            gateway push --file{" "}
            <Box component="span" sx={{ color: "#6EC1FF" }}>
              https://bijira.dev/samples/api.yaml
            </Box>
          </Box>

          <Tooltip title="Copy command">
            <IconButton
              onClick={() => copyCurl(g)}
              sx={{
                position: "absolute",
                top: 10,
                right: 6,
              }}
            >
              <ContentCopyOutlinedIcon />
            </IconButton>
          </Tooltip>
        </Box>

        {showApiDetails && (
          <Box sx={{ mt: 3 }}>
            <Typography variant="h6" mb={1} fontWeight={600} sx={{ mb: 1 }}>
              API Details
            </Typography>

            {(() => {
              const arr = apisData as ApiItem[];
              const api = arr?.[0];
              if (!api) return null;

              const typeLabel = api.tags?.[0] || "General";

              // WRAPPER enables hover + absolute positioning for the button
              return (
                <Box
                  sx={{
                    position: "relative",
                    "&:hover .testBtn": {
                      opacity: 1,
                      transform: "translateY(-50%) translateX(0)",
                    },
                  }}
                >
                  <Card testId={""}>
                    <CardContent>
                      <Stack direction="row" spacing={2} alignItems="center">
                        {/* Thumbnail */}
                        <Box
                          sx={{
                            width: 85,
                            height: 65,
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
                            fontWeight: 800,
                            fontSize: 20,
                            display: "flex",
                            alignItems: "center",
                            justifyContent: "center",
                          }}
                        >
                          {twoLetters(api.name)}
                        </Box>

                        {/* Text */}
                        <Box maxWidth={700} flex={1}>
                          <Stack
                            direction="row"
                            spacing={1}
                            alignItems="center"
                          >
                            <Typography variant="h6" sx={{ lineHeight: 1.2 }}>
                              {api.name}
                            </Typography>
                            <Chip
                              size="small"
                              label={typeLabel}
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
                      </Stack>
                    </CardContent>
                  </Card>

                  {/* Hover-only Test button, right-center */}
                  <Button
                    variant="outlined"
                    size="medium"
                    className="testBtn"
                    onClick={onGoStep3}
                    startIcon={<ScienceOutlinedIcon fontSize="small" />}
                    sx={{
                      position: "absolute",
                      top: "50%",
                      right: 16,
                      transform: "translateY(-50%) translateX(8px)",
                      opacity: 0,
                      transition: "opacity 180ms ease, transform 180ms ease",
                      pointerEvents: "auto",
                      "@media (hover: none)": {
                        opacity: 1,
                        transform: "translateY(-50%)",
                      },
                    }}
                  >
                    Test
                  </Button>
                </Box>
              );
            })()}
          </Box>
        )}
      </Box>
    </Box>
  );
}
