import React from "react";
import {
  Box,
  Divider,
  Grid,
  Paper,
  Stack,
  Typography,
  Chip,
  Tooltip,
  IconButton,
} from "@mui/material";
import AccessTimeIcon from "@mui/icons-material/AccessTime";
import ContentCopyIcon from "@mui/icons-material/ContentCopy";
import { Button } from "../../../components/src/components/Button";

import type { Gateway } from "../../../hooks/gateways";
import type { ApiSummary, ApiGatewaySummary } from "../../../hooks/apis";
import type { DeployRevisionResponseItem } from "../../../hooks/deployments";

const Card: React.FC<React.PropsWithChildren<{ sx?: any }>> = ({
  children,
  ...props
}) => (
  <Paper
    elevation={0}
    {...props}
    sx={{
      p: 3,
      borderRadius: 3,
      border: (t) => `1px solid ${t.palette.divider}`,
      width: 380,
      minHeight: 380, // increased height for Gateway URL section
      display: "flex",
      flexDirection: "column",
      justifyContent: "space-between",
      ...(props as any).sx,
    }}
  >
    {children}
  </Paper>
);

type Props = {
  gw: Gateway;
  apiId: string;
  api: ApiSummary | null;
  deployedMap: Map<string, ApiGatewaySummary>;
  deployByGateway: Record<string, DeployRevisionResponseItem>;
  deploying: boolean;
  deployingIds: Set<string>;
  relativeTime: (d?: string | Date | null) => string;
  onDeploy: (gatewayId: string) => void;
};

const GatewayDeployCard: React.FC<Props> = ({
  gw,
  apiId,
  api,
  deployedMap,
  deployByGateway,
  deploying,
  deployingIds,
  relativeTime,
  onDeploy,
}) => {
  const isDeployed = deployedMap.has(gw.id);
  const seededItem = deployByGateway[gw.id];
  const item = isDeployed ? seededItem : undefined;

  const deployedTime =
    item?.successDeployedTime ??
    item?.deployedTime ??
    gw.updatedAt ??
    gw.createdAt ??
    null;

  const vhost = gw.vhost || (item && item.vhost) || "";
  const description = gw.description || "";

  const status = (item?.status || "").toString().toUpperCase() as
    | "ACTIVE"
    | "CREATED"
    | "FAILED"
    | "IN_PROGRESS"
    | "ROLLED_BACK"
    | "UNKNOWN"
    | "";

  const success =
    status === "ACTIVE" || status === "CREATED" || status === "IN_PROGRESS";

  const title = gw.displayName || gw.name || "Gateway";
  const isDeployingThis = deploying || deployingIds.has(gw.id);

  // Construct Gateway URLs
  const httpUrl = api && vhost 
    ? `http://${vhost}:8080${api.context}/${api.version}`
    : null;
  const httpsUrl = api && vhost 
    ? `https://${vhost}:5443${api.context}/${api.version}`
    : null;

  const [copiedUrl, setCopiedUrl] = React.useState<string | null>(null);

  const handleCopyUrl = (url: string, protocol: string) => {
    navigator.clipboard.writeText(url).then(() => {
      setCopiedUrl(protocol);
      setTimeout(() => setCopiedUrl(null), 2000);
    });
  };

  return (
    <Grid key={gw.id}>
      <Card>
        <Box>
          {/* Header */}
          <Stack
            direction="row"
            justifyContent="space-between"
            alignItems="center"
            minWidth={300}
          >
            <Typography fontWeight={800}>{title}</Typography>
          </Stack>

          <Divider sx={{ my: 2 }} />

          {/* Deployed row */}
          <Stack direction="row" spacing={1} alignItems="center" mb={1}>
            <Typography>Deployed</Typography>
            <AccessTimeIcon fontSize="small" sx={{ opacity: 0.7 }} />
            <Typography color="text.info">
              {isDeployed
                ? deployedTime
                  ? relativeTime(deployedTime)
                  : "—"
                : "Not Deployed"}
            </Typography>
          </Stack>

          {!!vhost && (
            <Typography color="text.info" sx={{ mb: 0.5 }}>
              vhost: {vhost}
            </Typography>
          )}
          <Box height={12}>
            {!!description && (
              <Tooltip title={description} placement="bottom-start">
                <Typography
                  variant="body2"
                  color="#afa9a9ff"
                  sx={{
                    mb: 1,
                    whiteSpace: "nowrap",
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    maxWidth: 340,
                    display: "block",
                  }}
                >
                  {description}
                </Typography>
              </Tooltip>
            )}
          </Box>

          <Box
            mt={2}
            sx={{
              backgroundColor: (t) =>
                !isDeployed
                  ? t.palette.mode === "dark"
                    ? "rgba(107,114,128,0.12)" // neutral gray for not deployed
                    : "#F4F4F5"
                  : success
                  ? t.palette.mode === "dark"
                    ? "rgba(16,185,129,0.12)"
                    : "#E8F7EC"
                  : t.palette.mode === "dark"
                  ? "rgba(239,68,68,0.12)"
                  : "#FDECEC",
              border: (t) =>
                `1px solid ${
                  !isDeployed
                    ? t.palette.divider
                    : success
                    ? "#D8EEDC"
                    : t.palette.error.light
                }`,
              borderRadius: 2,
              px: 2,
              py: 1.25,
              mb: 2,
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <Typography fontWeight={500}>Deployment Status</Typography>
            <Chip
              label={isDeployed ? status || "ACTIVE" : "NOT DEPLOYED"}
              color={!isDeployed ? "default" : success ? "success" : "error"}
              variant={
                !isDeployed ? "outlined" : success ? "filled" : "outlined"
              }
              size="small"
            />
          </Box>

          {/* Gateway URLs section */}
          {httpUrl && httpsUrl && (
            <Box mt={2} mb={3}>
              <Typography fontWeight={500} sx={{ mb: 1 }}>
                Gateway URL
              </Typography>
              <Stack spacing={1}>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    backgroundColor: (t) =>
                      t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "#F9FAFB",
                    borderRadius: 1,
                    px: 1.5,
                    py: 1,
                    border: (t) => `1px solid ${t.palette.divider}`,
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{
                      fontWeight: 600,
                      color: "text.secondary",
                      minWidth: "45px",
                    }}
                  >
                    HTTP
                  </Typography>
                  <Tooltip title={httpUrl} placement="top">
                    <Typography
                      variant="body2"
                      sx={{
                        flex: 1,
                        fontSize: "0.75rem",
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        fontFamily: "monospace",
                      }}
                    >
                      {httpUrl}
                    </Typography>
                  </Tooltip>
                  <Tooltip title={copiedUrl === "http" ? "Copied!" : "Copy HTTP URL"}>
                    <IconButton
                      size="small"
                      onClick={() => handleCopyUrl(httpUrl, "http")}
                    >
                      <ContentCopyIcon sx={{ fontSize: 16 }} />
                    </IconButton>
                  </Tooltip>
                </Box>
                <Box
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    gap: 1,
                    backgroundColor: (t) =>
                      t.palette.mode === "dark" ? "rgba(255,255,255,0.05)" : "#F9FAFB",
                    borderRadius: 1,
                    px: 1.5,
                    py: 1,
                    border: (t) => `1px solid ${t.palette.divider}`,
                  }}
                >
                  <Typography
                    variant="caption"
                    sx={{
                      fontWeight: 600,
                      color: "text.secondary",
                      minWidth: "45px",
                    }}
                  >
                    HTTPS
                  </Typography>
                  <Tooltip title={httpsUrl} placement="top">
                    <Typography
                      variant="body2"
                      sx={{
                        flex: 1,
                        fontSize: "0.75rem",
                        whiteSpace: "nowrap",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        fontFamily: "monospace",
                      }}
                    >
                      {httpsUrl}
                    </Typography>
                  </Tooltip>
                  <Tooltip title={copiedUrl === "https" ? "Copied!" : "Copy HTTPS URL"}>
                    <IconButton
                      size="small"
                      onClick={() => handleCopyUrl(httpsUrl, "https")}
                    >
                      <ContentCopyIcon sx={{ fontSize: 16 }} />
                    </IconButton>
                  </Tooltip>
                </Box>
              </Stack>
            </Box>
          )}
        </Box>

        {/* Deploy / Re-deploy action (pinned to bottom) */}
        <Button
          variant="contained"
          fullWidth
          disabled={!apiId || isDeployingThis}
          onClick={() => onDeploy(gw.id)}
        >
          {isDeployed
            ? isDeployingThis
              ? "Re-deploying…"
              : "Re-deploy"
            : isDeployingThis
            ? "Deploying…"
            : "Deploy"}
        </Button>
      </Card>
    </Grid>
  );
};

export default GatewayDeployCard;
