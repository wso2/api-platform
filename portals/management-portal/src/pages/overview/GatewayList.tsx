import { Box, Typography, Stack, Tooltip, colors } from "@mui/material";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";

import type { GatewayRecord } from "./types";
import { codeFor, relativeTime, twoLetters } from "./utils";
import { Card } from "../../components/src/components/Card";
import { Chip } from "../../components/src/components/Chip";
import { IconButton } from "../../components/src/components/IconButton";
import Edit from "../../components/src/Icons/generated/Edit";
import Delete from "../../components/src/Icons/generated/Delete";
import Copy from "../../components/src/Icons/generated/Copy";

export default function GatewayList({
  items,
  onCopy,
  onEdit,
  onDelete,
  onAdd,
}: {
  items: GatewayRecord[];
  onAdd: () => void;
  onEdit: (g: GatewayRecord) => void;
  onDelete: (id: string) => void;
  onCopy: (g: GatewayRecord) => void;
}) {
  return (
    <>
      {items.map((g) => {
        const cmd = codeFor(g.name);
        return (
          <Card
            key={g.id}
            testId={""}
            style={{ padding: 16, marginBottom: 16 }}
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
              {/* Left block */}
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  gap: 2,
                  flex: 1,
                }}
              >
                {/* Thumbnail */}
                <Box
                  sx={(theme) => ({
                    width: 85,
                    height: 85,
                    borderRadius: 1,
                    backgroundImage: `linear-gradient(135deg,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).light} 0%,
    #059669 55%,
    ${theme.palette.augmentColor({ color: { main: "#059669" } }).dark} 100%)`,
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

                {/* Text */}
                <Box sx={{ flex: 1 }}>
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Chip
                      label={g.type === "hybrid" ? "On Premise" : "Cloud"}
                      color="info"
                      variant="outlined"
                      sx={{ borderRadius: 1 }}
                    />
                    {/* Active chip appears only AFTER user copies the curl command */}
                  </Stack>
                  <Typography variant="h5" fontWeight={800} sx={{ mt: 0.5 }}>
                    {g.displayName}
                  </Typography>
                  {g.description && (
                    <Typography
                      color="text.secondary"
                      sx={{ mt: 0.2, maxWidth: 900 }}
                    >
                      {g.description}
                    </Typography>
                  )}
                </Box>
              </Box>

              {/* Actions */}
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

            {/* Meta rows */}
            <Box>
              <Box display={"flex"} gap={4} mt={2} alignItems={"center"}>
                <Typography color="text.disabled">Created:</Typography>
                <Stack
                  direction="row"
                  spacing={1}
                  alignItems="center"
                  sx={{ mt: 0.5 }}
                >
                  <AccessTimeIcon
                    fontSize="small"
                    sx={{ color: "text.secondary" }}
                  />
                  <Typography color="text.secondary">
                    {relativeTime(g.createdAt)}
                  </Typography>
                  {g.isActive && (
                    <Chip
                      label="Active"
                      // color="success"
                      variant="outlined"
                      sx={{ borderRadius: 1 }}
                    />
                  )}
                </Stack>
              </Box>

              <Box display={"flex"} gap={4} alignItems={"center"}>
                <Typography color="text.disabled">Host:</Typography>
                <Typography sx={{ mt: 0.5 }}>{g.host || "-"}</Typography>
              </Box>
              <Box display={"flex"} gap={4} mb={1} alignItems={"center"}>
                <Typography color="text.disabled">Replicas:</Typography>
                <Typography sx={{ mt: 0.5 }}>02</Typography>
              </Box>
            </Box>

            {/* Command block */}
            <Box sx={{ mt: 3, position: "relative" }}>
              <Typography variant="body2" sx={{ mb: 1 }}>
                Run This Command locally to start the gateway
              </Typography>

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
                <Box
                  component="span"
                  sx={{ color: "#C5E478", fontWeight: 700 }}
                >
                  curl
                </Box>{" "}
                -Ls{" "}
                <Box component="span" sx={{ color: "#6EC1FF" }}>
                  https://bijira.dev/quick-start
                </Box>{" "}
                |{" "}
                <Box
                  component="span"
                  sx={{ color: "#E8D06B", fontWeight: 700 }}
                >
                  bash
                </Box>{" "}
                -s -- -k{" "}
                <Box component="span" sx={{ color: "#7FE0E7" }}>
                  $GATEWAY_KEY
                </Box>{" "}
                --name{" "}
                <Box component="span" sx={{ color: "#F2A65A" }}>
                  {g.name}
                </Box>
              </Box>
              <Tooltip title="Copy command">
                <IconButton
                  variant="outlined"
                  onClick={() => onCopy(g)}
                  sx={{
                    position: "absolute",
                    top: 40,
                    right: 6,
                  }}
                >
                  <Copy />
                </IconButton>
              </Tooltip>
            </Box>
          </Card>
        );
      })}
    </>
  );
}
