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
import type { ApiSummary, ApiGatewaySummary } from "../../hooks/apis";

import AccessTimeIcon from "@mui/icons-material/AccessTime";
import StopOutlinedIcon from "@mui/icons-material/StopOutlined";
import { Button } from "../../components/src/components/Button";

import {
  DeploymentProvider,
  useDeployment,
} from "../../context/DeploymentContext";
import type { DeployRevisionResponseItem } from "../../hooks/deployments";

import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import type { Gateway } from "../../hooks/gateways";

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
  const { fetchApiById, fetchGatewaysForApi } = useApisContext();
  const { gateways, loading: gatewaysLoading } = useGateways();
  const { deployApiRevision, loading: deploying } = useDeployment();

  const [params] = useSearchParams();
  const apiId = params.get("apiId") ?? "";
  const revisionIdFromQuery = params.get("revisionId") ?? "7";

  const [api, setApi] = React.useState<ApiSummary | null>(null);

  // gateways already deployed for THIS api
  const [deployedForApi, setDeployedForApi] = React.useState<
    ApiGatewaySummary[]
  >([]);

  // seeded + live deployment items keyed by gatewayId
  const [deployByGateway, setDeployByGateway] = React.useState<
    Record<string, DeployRevisionResponseItem>
  >({});

  const [loading, setLoading] = React.useState(true);
  const [deployingIds, setDeployingIds] = React.useState<Set<string>>(
    () => new Set()
  );

  React.useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        if (!apiId) return;

        const [apiData, apiDeployedGws] = await Promise.all([
          fetchApiById(apiId),
          fetchGatewaysForApi(apiId),
        ]);

        if (!mounted) return;

        setApi(apiData);
        setDeployedForApi(apiDeployedGws);

        // Seed deployed gateways so they initially show the status section
        const nowIso = new Date().toISOString();
        const seeded: Record<string, DeployRevisionResponseItem> = {};
        apiDeployedGws.forEach((gw) => {
          seeded[gw.id] = {
            gatewayId: gw.id,
            revisionId: String(revisionIdFromQuery),
            vhost: gw.vhost ?? undefined,
            status: "ACTIVE",
            deployedTime: nowIso,
            successDeployedTime: nowIso,
          } as DeployRevisionResponseItem;
        });
        setDeployByGateway(seeded);
      } finally {
        if (mounted) setLoading(false);
      }
    })();
    return () => {
      mounted = false;
    };
  }, [apiId, fetchApiById, fetchGatewaysForApi, revisionIdFromQuery]);

  const deployedMap = React.useMemo(() => {
    const m = new Map<string, ApiGatewaySummary>();
    deployedForApi.forEach((g) => m.set(g.id, g));
    return m;
  }, [deployedForApi]);

  const gatewaysById = React.useMemo(() => {
    const m = new Map<string, Gateway>();
    gateways.forEach((g) => m.set(g.id, g));
    return m;
  }, [gateways]);

  const handleDeploySingle = async (gatewayId: string) => {
    if (!apiId) return;
    const gw = gatewaysById.get(gatewayId);
    const targets = [
      {
        gatewayId,
        vhost: gw?.vhost ?? undefined,
        displayOnDevportal: true,
      },
    ];

    try {
      setDeployingIds((prev) => new Set(prev).add(gatewayId));
      const resp = await deployApiRevision(apiId, revisionIdFromQuery, targets);
      const item = resp.find((x) => x.gatewayId === gatewayId) ?? resp[0];

      if (item) {
        setDeployByGateway((prev) => ({ ...prev, [gatewayId]: item }));

        // if it wasn't deployed before, mark it as deployed for this API now
        if (!deployedMap.has(gatewayId) && gw) {
          const newly: ApiGatewaySummary = {
            id: gw.id,
            organizationId: (gw as any).organizationId ?? "",
            name: gw.name,
            displayName: gw.displayName ?? undefined,
            description: gw.description ?? undefined,
            vhost: gw.vhost ?? undefined,
            // 🔴 CRITICAL comes from Gateways list (not deployed info)
            isCritical:
              (gw as any).isCritical ??
              (gw as any).critical ??
              false,
            functionalityType: (gw as any).functionalityType ?? "regular",
            isActive: true,
            createdAt: (gw as any).createdAt ?? undefined,
            updatedAt: new Date().toISOString(),
          };
          setDeployedForApi((prev) => [...prev, newly]);
        }
      }
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
            const isDeployed = deployedMap.has(gw.id);
            const seededItem = deployByGateway[gw.id];
            const item = isDeployed ? seededItem : undefined;

            const apiDeployed = deployedMap.get(gw.id);

            const lastWhen =
              item?.successDeployedTime ||
              item?.deployedTime ||
              gw.updatedAt ||
              gw.createdAt ||
              null;

            const vhost = gw.vhost || (item && item.vhost) || "";
            const description = gw.description || "";

            // ✅ CRITICAL strictly based on gateways list data,
            // and shown regardless of deployed state (before and after).
            const isCritical =
              (gw as any)?.isCritical === true ||
              (gw as any)?.critical === true;

            // Active flag only used for other UI choices if needed later
            const isActive = apiDeployed?.isActive !== false;

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
                    <Stack direction="row" alignItems="center" spacing={1.25}>
                      {isCritical && (
                        <Chip
                          label="Critical"
                          color="error"
                          size="small"
                          variant="outlined"
                          sx={{ height: 22 }}
                        />
                      )}
                      <Typography fontWeight={800}>{title}</Typography>
                    </Stack>

                    {/* If you want Stop for deployed gateways, uncomment this */}
                    {/* {isDeployed && (
                      <Button
                        size="small"
                        variant="outlined"
                        color="error"
                        startIcon={<StopOutlinedIcon />}
                      >
                        Stop
                      </Button>
                    )} */}
                  </Stack>

                  <Divider sx={{ my: 2 }} />

                  {/* Deployed row */}
                  <Stack direction="row" spacing={1} alignItems="center" mb={1}>
                    <Typography>Deployed</Typography>
                    <AccessTimeIcon fontSize="small" sx={{ opacity: 0.7 }} />
                    <Typography color="text.secondary">
                      {isDeployed
                        ? lastWhen
                          ? relativeTime(lastWhen)
                          : "0 min ago"
                        : "Not Deployed"}
                    </Typography>
                  </Stack>

                  {vhost && (
                    <Typography color="text.secondary" sx={{ mb: 0.5 }}>
                      vhost: {vhost}
                    </Typography>
                  )}

                  {description && (
                    <Typography
                      variant="body2"
                      color="text.secondary"
                      sx={{ mb: 1 }}
                    >
                      {description}
                    </Typography>
                  )}

                  {isDeployed && (
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
                        label={status || "ACTIVE"}
                        color={success ? "success" : "error"}
                        variant={success ? "filled" : "outlined"}
                        size="small"
                      />
                    </Box>
                  )}

                  {/* Deploy / Re-deploy action */}
                  <Button
                    variant="contained"
                    fullWidth
                    disabled={!apiId || isDeployingThis}
                    onClick={() => handleDeploySingle(gw.id)}
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
