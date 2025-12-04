import React from "react";
import { Box, Chip, Skeleton, Stack, Tooltip, Typography } from "@mui/material";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import CachedRoundedIcon from "@mui/icons-material/CachedRounded";

import { Card } from "../../components/src/components/Card";
import Edit from "../../components/src/Icons/generated/Edit";
import Delete from "../../components/src/Icons/generated/Delete";
import Copy from "../../components/src/Icons/generated/Copy";

import ReactMarkdown from "react-markdown";
import type { Components } from "react-markdown";
import type { Gateway } from "../../hooks/gateways";
import { IconButton } from "../../components/src/components/IconButton";
import { Button } from "../../components/src/components/Button";
import { AddIcon } from "../../components/src/Icons";

type Props = {
  gateways: Gateway[];
  gatewayTokens: Record<string, { token?: string }>;
  getGatewayStatus?: (id: string) => { isActive?: boolean } | undefined;
  gatewayStatuses?: Record<string, { isActive?: boolean }>;
  onAddClick: () => void;
  onEdit: (g: Gateway) => void;
  onDelete: (id: string) => Promise<void> | void;
  onRotateToken: (id: string) => Promise<void> | void;
  manualRotatingId: string | null;
  mdComponentsForCmd: Components;
  cmdTextDisplayMd: string;
  onCopy: (text: string) => Promise<void> | void;
  codeFor: (gatewayName: string, token?: string) => string;
  relativeTime: (d?: string | Date | null) => string;
  /** NEW: show skeletons while data is loading */
  loading?: boolean;
};

const twoLetters = (s: string) => {
  const letters = (s || "").replace(/[^A-Za-z]/g, "");
  if (!letters) return "GW";
  const first = letters[0]?.toUpperCase() ?? "";
  const second = letters[1]?.toLowerCase() ?? "";
  return `${first}${second}`;
};

const truncate = (text: string | undefined | null, max: number) => {
  const t = (text ?? "").trim();
  if (t.length <= max) return { shown: t, truncated: false };
  return { shown: t.slice(0, Math.max(0, max - 1)) + "…", truncated: true };
};

const SKELETON_COUNT = 4;
const MAX_DESC_CHARS = 200;

const GatewayList: React.FC<Props> = ({
  gateways,
  gatewayTokens,
  getGatewayStatus,
  gatewayStatuses,
  onAddClick,
  onEdit,
  onDelete,
  onRotateToken,
  manualRotatingId,
  mdComponentsForCmd,
  cmdTextDisplayMd,
  onCopy,
  codeFor,
  relativeTime,
  loading,
}) => {
  return (
    <>
      <Box
        mb={2}
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
        }}
      >
        <Typography variant="h3" fontWeight={600}>
          Gateways
        </Typography>
        <Box>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            onClick={onAddClick}
            sx={{ bgcolor: "primary.dark", textTransform: "none" }}
          >
            Add
          </Button>
        </Box>
      </Box>

      {/* Skeletons while loading */}
      {loading &&
        Array.from({ length: SKELETON_COUNT }).map((_, i) => (
          <Card key={`sk-${i}`} style={{ padding: 20, marginBottom: 10 }} testId={""}>
            {/* Header row */}
            <Box
              sx={{
                display: "flex",
                alignItems: "flex-start",
                justifyContent: "space-between",
                gap: 2,
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
                <Skeleton
                  variant="rounded"
                  width={85}
                  height={85}
                  sx={{ borderRadius: 1 }}
                />
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Skeleton variant="text" width={100} height={24} />
                  <Skeleton variant="text" width="60%" height={28} />
                  <Skeleton variant="text" width="90%" height={18} />
                </Box>
              </Box>
              <Stack direction="row" spacing={1}>
                <Skeleton variant="circular" width={36} height={36} />
                <Skeleton variant="circular" width={36} height={36} />
              </Stack>
            </Box>

            {/* Meta */}
            <Box sx={{ mt: 2 }}>
              <Box display="flex" gap={4} alignItems="center">
                <Skeleton variant="text" width={80} height={18} />
                <Skeleton variant="text" width={120} height={18} />
                <Skeleton variant="rounded" width={70} height={24} />
              </Box>
              <Box display="flex" gap={4} alignItems="center" mt={1}>
                <Skeleton variant="text" width={60} height={18} />
                <Skeleton variant="text" width={200} height={18} />
              </Box>
              <Box display="flex" gap={4} alignItems="center" mt={1}>
                <Skeleton variant="text" width={70} height={18} />
                <Skeleton variant="text" width={30} height={18} />
              </Box>
            </Box>

            {/* Command block */}
            <Box sx={{ mt: 3 }}>
              <Skeleton variant="text" width={260} height={18} />
              <Skeleton
                variant="rounded"
                height={110}
                sx={{ borderRadius: 1, mt: 1 }}
              />
            </Box>
          </Card>
        ))}

      {/* Actual list (hidden while loading) */}
      {!loading &&
        gateways.map((g) => {
          const latestToken = gatewayTokens[g.id]?.token;
          const hasToken = Boolean(latestToken);
          const status =
            typeof getGatewayStatus === "function"
              ? getGatewayStatus(g.id)
              : gatewayStatuses?.[g.id];

          const isActive = status?.isActive === true;
          const cmdTextCopy =
            `curl -sLO https://github.com/wso2/api-platform/releases/download/gateway-v0.0.1/gateway-v0.0.1.zip && \\\n` +
            `unzip gateway-v0.0.1.zip && \\\n` +
            `GATEWAY_CONTROLPLANE_TOKEN=${latestToken ?? ""} ` +
            `docker compose --project-directory gateway up`;
          const cmd = hasToken
            ? codeFor(g.name, latestToken)
            : "Rotate the gateway token to generate the command.";

          const desc = truncate(g.description, MAX_DESC_CHARS);

          return (
            <Card
              key={g.id}
              testId={""}
              style={{ padding: 20, marginBottom: 10 }}
            >
              {/* Header row */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  gap: 2,
                }}
              >
                {/* Left: avatar + title/desc */}
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "flex-start",
                    gap: 2,
                    flex: 1,
                  }}
                >
                  <Box
                    sx={(theme) => ({
                      width: 85,
                      height: 85,
                      borderRadius: 1,
                      backgroundImage: `linear-gradient(135deg,
        ${theme.palette.augmentColor({ color: { main: "#059669" } }).light} 0%,
        #059669 55%,
        ${
          theme.palette.augmentColor({ color: { main: "#059669" } }).dark
        } 100%)`,
                      color: "common.white",
                      fontWeight: 800,
                      fontSize: 28,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "center",
                    })}
                    aria-label={`${g.displayName} thumbnail`}
                  >
                    {twoLetters(g.displayName || g.name)}
                  </Box>

                  {/* Title + badge + description */}
                  <Box sx={{ flex: 1, minWidth: 0 }}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Chip
                        size="small"
                        label={
                          (g.type ?? "hybrid") === "hybrid"
                            ? "On Premise"
                            : "Cloud"
                        }
                        color="info"
                        variant="outlined"
                        style={{ borderRadius: 4 }}
                      />
                    </Stack>
                    <Typography fontSize={18} fontWeight={600} sx={{ mt: 0.5 }}>
                      {g.displayName}
                    </Typography>

                    {desc.shown &&
                      (desc.truncated ? (
                        <Tooltip title={g.description} placement="top">
                          <Typography
                            color="#959595"
                            sx={{ mt: 0.2, maxWidth: 900 }}
                          >
                            {desc.shown}
                          </Typography>
                        </Tooltip>
                      ) : (
                        <Typography
                          color="#959595"
                          sx={{ mt: 0.2, maxWidth: 900 }}
                        >
                          {desc.shown}
                        </Typography>
                      ))}
                  </Box>
                </Box>

                {/* Right: actions */}
                <Stack direction="row" spacing={1}>
                  <Tooltip title="Edit">
                    <IconButton color="primary" onClick={() => onEdit(g)}>
                      <Edit />
                    </IconButton>
                  </Tooltip>
                  <Tooltip title="Delete">
                    <IconButton color="error" onClick={() => onDelete(g.id)}>
                      <Delete />
                    </IconButton>
                  </Tooltip>
                </Stack>
              </Box>

              {/* Meta */}
              <Box>
                <Box display={"flex"} gap={4} mt={2}>
                  <Typography color="text.disabled">Created:</Typography>
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    sx={{ mt: 0.5 }}
                  >
                    <AccessTimeIcon
                      fontSize="small"
                      sx={{ color: "#959595" }}
                    />
                    <Typography color="#959595">
                      {relativeTime(g.createdAt)}
                    </Typography>
                    {isActive ? (
                      <Chip
                        size="small"
                        label="Active"
                        color="success"
                        variant="outlined"
                        style={{ borderRadius: 4 }}
                      />
                    ) : (
                      <Chip
                        size="small"
                        label="Not Active"
                        color="error"
                        variant="outlined"
                        style={{ borderRadius: 4 }}
                      />
                    )}
                  </Stack>
                </Box>
                <Box display={"flex"} gap={4} alignItems={"center"}>
                  <Typography color="text.disabled">Host:</Typography>
                  <Typography sx={{ mt: 0.5 }}>
                    {g.vhost ?? g.host ?? "-"}
                  </Typography>
                </Box>
                <Box display={"flex"} gap={4} mb={1} alignItems={"center"}>
                  <Typography color="text.disabled">Replicas:</Typography>
                  <Typography sx={{ mt: 0.5 }}>01</Typography>
                </Box>
              </Box>

              {/* Command block */}
              <Box sx={{ mt: 3 }}>
                <Typography variant="body2" sx={{ mb: 1 }}>
                  Run This Command locally to start the gateway
                </Typography>

                <Box sx={{ position: "relative" }}>
                  <ReactMarkdown components={mdComponentsForCmd}>
                    {cmdTextDisplayMd}
                  </ReactMarkdown>

                  <Box
                    sx={{
                      position: "absolute",
                      top: 10,
                      right: 6,
                      display: "flex",
                      gap: 1,
                    }}
                  >
                    <Tooltip title="Rotate token">
                      <IconButton
                        variant="outlined"
                        onClick={() => onRotateToken(g.id)}
                        disabled={manualRotatingId === g.id}
                        size="small"
                      >
                        <CachedRoundedIcon
                          style={{ color: "#fff", fill: "#fff" }}
                        />
                      </IconButton>
                    </Tooltip>

                    <Tooltip
                      title={hasToken ? "Copy command" : "Rotate token first"}
                    >
                      <span>
                        <IconButton
                          variant="outlined"
                          onClick={() => onCopy(cmdTextCopy)}
                          disabled={!hasToken}
                          size="small"
                        >
                          <Copy style={{ color: "#fff", fill: "#fff" }} />
                        </IconButton>
                      </span>
                    </Tooltip>
                  </Box>
                </Box>
              </Box>
            </Card>
          );
        })}
    </>
  );
};

export default GatewayList;
