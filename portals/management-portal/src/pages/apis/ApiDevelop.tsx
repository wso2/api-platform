import React from "react";
import {
  Box,
  Chip,
  CircularProgress,
  Divider,
  Grid,
  Paper,
  Stack,
  Typography,
} from "@mui/material";
import { useSearchParams } from "react-router-dom";

import { ApiProvider, useApisContext } from "../../context/ApiContext";
import type { ApiSummary } from "../../hooks/apis";

import AccessTimeIcon from "@mui/icons-material/AccessTime";
import StopOutlinedIcon from "@mui/icons-material/StopOutlined";
import { Button } from "../../components/src/components/Button";

import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import type { Gateway } from "../../hooks/gateways";

import {
  DeploymentProvider,
  useDeployment,
} from "../../context/DeploymentContext";
import type { DeployRevisionResponseItem } from "../../hooks/deployments";

/* ---------------- helpers ---------------- */

const relativeTime = (d?: string | Date | null) => {
  if (!d) return "";
  const date = typeof d === "string" ? new Date(d) : d;
  const diff = Math.max(0, Date.now() - date.getTime());
  const min = Math.floor(diff / 60000);
  const hr = Math.floor(min / 60);
  const mo = Math.floor(hr / (24 * 30));
  if (mo >= 1) return `${mo} month${mo > 1 ? "s" : ""} ago`;
  if (hr >= 24) {
    const dd = Math.floor(hr / 24);
    return `${dd} day${dd > 1 ? "s" : ""} ago`;
  }
  if (hr >= 1) return `${hr} hr${hr > 1 ? "s" : ""} ago`;
  return `${min} min ago`;
};

const Card = (props: any) => (
  <Paper
    elevation={0}
    {...props}
    sx={{
      p: 3,
      borderRadius: 3,
      border: (t) => `1px solid ${t.palette.divider}`,
      width: 380, // fixed width so all cards align
      ...props.sx,
    }}
  />
);

/* ---------------- page content ---------------- */

const DevelopContent: React.FC = () => {
  const { fetchApiById } = useApisContext();
  const { gateways, loading: gatewaysLoading } = useGateways();
  const { deployApiRevision, loading: deploying } = useDeployment();

  const [params] = useSearchParams();
  const apiId = params.get("apiId") ?? "";
  const revisionIdFromQuery = params.get("revisionId") ?? "7";

  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [loading, setLoading] = React.useState(true);

  const [deployByGateway, setDeployByGateway] = React.useState<
    Record<string, DeployRevisionResponseItem>
  >({});

  const [deployingIds, setDeployingIds] = React.useState<Set<string>>(
    () => new Set()
  );

  React.useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        if (!apiId) return;
        const data = await fetchApiById(apiId);
        if (mounted) setApi(data);
      } finally {
        if (mounted) setLoading(false);
      }
    })();
    return () => {
      mounted = false;
    };
  }, [apiId, fetchApiById]);

  const gatewaysById = React.useMemo(() => {
    const map = new Map<string, Gateway>();
    gateways.forEach((g) => map.set(g.id, g));
    return map;
  }, [gateways]);

  const handleDeploySingle = async (gatewayId: string) => {
    if (!apiId) return;
    const gw = gatewaysById.get(gatewayId);
    const targets = [
      {
        gatewayId,
        vhost: gw?.vhost || (gw as any)?.host || undefined,
        displayOnDevportal: true,
      },
    ];

    try {
      setDeployingIds((prev) => new Set(prev).add(gatewayId));
      const resp = await deployApiRevision(apiId, revisionIdFromQuery, targets);
      const item = resp.find((x) => x.gatewayId === gatewayId) ?? resp[0];
      if (item) setDeployByGateway((prev) => ({ ...prev, [gatewayId]: item }));
    } catch {
      // error surfaced via context
    } finally {
      setDeployingIds((prev) => {
        const next = new Set(prev);
        next.delete(gatewayId);
        return next;
      });
    }
  };

  if (loading || gatewaysLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box>
      {gateways.length === 0 ? (
        <Card sx={{ mt: 2 }}>
          <Typography color="text.secondary">No gateways found.</Typography>
        </Card>
      ) : (
        <Grid container spacing={3}>
          {gateways.map((gw) => {
            const item = deployByGateway[gw.id];

            // data to show (some may be absent)
            const lastWhen =
              item?.successDeployedTime ||
              item?.deployedTime ||
              (gw as any)?.lastDeployedAt ||
              null;

            const vhost =
              gw?.vhost || (gw as any)?.host || (item && item.vhost) || "";

            const description =
              gw?.description || (gw as any)?.summary || "";

            const isCritical =
              (gw as any)?.critical === true ||
              (gw as any)?.isCritical === true ||
              String((gw as any)?.criticality || "")
                .toLowerCase()
                .includes("critical") ||
              String((gw as any)?.tier || "").toLowerCase() === "critical";

            const status = (item?.status || "").toString().toUpperCase() as
              | "ACTIVE"
              | "CREATED"
              | "FAILED"
              | "IN_PROGRESS"
              | "ROLLED_BACK"
              | "UNKNOWN"
              | "";

            const success =
              status === "ACTIVE" ||
              status === "CREATED" ||
              status === "IN_PROGRESS";

            const title = gw.displayName || gw.name || "Gateway";
            const isDeployingThis = deploying || deployingIds.has(gw.id);

            return (
              <Grid key={gw.id}>
                <Card>
                  {/* Header */}
                  <Stack
                    direction="row"
                    justifyContent="space-between"
                    alignItems="center"
                    minWidth={300}
                  >
                    <Stack direction="row" alignItems="center" spacing={1}>
                      {isCritical && (
                        <Box
                          sx={{
                            width: 10,
                            height: 10,
                            borderRadius: "50%",
                            bgcolor: (t) => t.palette.error.main,
                          }}
                        />
                      )}
                      <Typography fontWeight={800}>{title}</Typography>
                    </Stack>

                    {item && (
                      <Button
                        size="small"
                        variant="outlined"
                        color="error"
                        startIcon={<StopOutlinedIcon />}
                      >
                        Stop
                      </Button>
                    )}
                  </Stack>

                  <Divider sx={{ my: 2 }} />

                  {/* Deployed row (always shown). If missing -> "Not Deployed" */}
                  <Stack direction="row" spacing={1} alignItems="center" mb={1}>
                    <Typography>Deployed</Typography>
                    <AccessTimeIcon fontSize="small" sx={{ opacity: 0.7 }} />
                    <Typography color="text.secondary">
                      {lastWhen ? relativeTime(lastWhen) : "Not Deployed"}
                    </Typography>
                  </Stack>

                  {/* vhost: only render if present */}
                  {vhost && (
                    <Typography color="text.secondary" sx={{ mb: 0.5 }}>
                      vhost: {vhost}
                    </Typography>
                  )}

                  {/* description: only render if present */}
                  {description && (
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 1 }}
                    >
                      {description}
                    </Typography>
                  )}

                  {/* Status banner only after deployed */}
                  {item && (
                    <Box
                      mt={2}
                      sx={{
                        backgroundColor: (t) =>
                          success
                            ? t.palette.mode === "dark"
                              ? "rgba(16,185,129,0.12)"
                              : "#E8F7EC"
                            : t.palette.mode === "dark"
                            ? "rgba(239,68,68,0.12)"
                            : "#FDECEC",
                        border: (t) =>
                          `1px solid ${
                            success ? "#D8EEDC" : t.palette.error.light
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
                      <Typography fontWeight={500}>
                        Deployment Status
                      </Typography>
                      <Chip
                        label={status || "—"}
                        color={success ? "success" : "error"}
                        variant={success ? "filled" : "outlined"}
                        size="small"
                      />
                    </Box>
                  )}

                  {/* Deploy button visible only before first deploy */}
                  {!item && (
                    <Button
                      variant="contained"
                      fullWidth
                      disabled={!apiId || isDeployingThis}
                      onClick={() => handleDeploySingle(gw.id)}
                    >
                      {isDeployingThis ? "Deploying…" : "Deploy"}
                    </Button>
                  )}
                </Card>
              </Grid>
            );
          })}
        </Grid>
      )}
    </Box>
  );
};

/* Wrap with providers */
const ApiDevelop: React.FC = () => (
  <ApiProvider>
    <GatewayProvider>
      <DeploymentProvider>
        <DevelopContent />
      </DeploymentProvider>
    </GatewayProvider>
  </ApiProvider>
);

export default ApiDevelop;
