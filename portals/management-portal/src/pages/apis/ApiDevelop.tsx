import React from "react";
import {
  Box,
  Checkbox,
  Chip,
  CircularProgress,
  Divider,
  FormControlLabel,
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

// Gateways (real list)
import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import type { Gateway } from "../../hooks/gateways";

// Deployments
import {
  DeploymentProvider,
  useDeployment,
} from "../../context/DeploymentContext";
import type { DeployRevisionResponseItem } from "../../hooks/deployments";

/* ---------------- helpers ---------------- */

const relativeTime = (d?: string | Date | null) => {
  if (!d) return "—";
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
  const revisionIdFromQuery = params.get("revisionId") ?? "7"; // fallback

  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [loading, setLoading] = React.useState(true);

  // selection & deployed response state
  const [selectedGatewayIds, setSelectedGatewayIds] = React.useState<string[]>(
    []
  );

  // map of gatewayId -> server response item
  const [deployByGateway, setDeployByGateway] = React.useState<
    Record<string, DeployRevisionResponseItem>
  >({});

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

  const toggleId = (id: string, checked: boolean) =>
    setSelectedGatewayIds((prev) =>
      checked ? [...prev, id] : prev.filter((x) => x !== id)
    );

  const handleDeploy = async () => {
    if (!apiId || selectedGatewayIds.length === 0) return;

    // build targets using gatewayId + vhost + displayOnDevportal
    const targets = selectedGatewayIds.map((id) => {
      const gw = gatewaysById.get(id);
      return {
        gatewayId: id,
        vhost: gw?.vhost || gw?.host || undefined,
        displayOnDevportal: true,
      };
    });

    try {
      const resp = await deployApiRevision(apiId, revisionIdFromQuery, targets);
      // update local map by gatewayId (so UI can show per-gateway card)
      setDeployByGateway((prev) => {
        const next = { ...prev };
        resp.forEach((item) => {
          next[item.gatewayId] = item;
        });
        return next;
      });
    } catch {
      // errors are surfaced via DeploymentContext.error; keep UI unchanged
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
      <Grid container spacing={3}>
        {/* LEFT: Select Gateways */}
        <Grid>
          <Card>
            <Stack
              direction="row"
              justifyContent="space-between"
              alignItems="center"
              minWidth={300}
            >
              <Typography fontWeight={800}>Select Gateways</Typography>
            </Stack>

            <Divider sx={{ my: 2 }} />

            {/* checkbox list from real gateways */}
            <Stack spacing={1.25} sx={{ mt: 2 }}>
              {gateways.length === 0 ? (
                <Typography color="text.secondary">
                  No gateways found.
                </Typography>
              ) : (
                gateways.map((g) => {
                  const label = g.displayName || g.name;
                  return (
                    <FormControlLabel
                      key={g.id}
                      control={
                        <Checkbox
                          checked={selectedGatewayIds.includes(g.id)}
                          onChange={(e) => toggleId(g.id, e.target.checked)}
                        />
                      }
                      label={<Typography variant="body1">{label}</Typography>}
                    />
                  );
                })
              )}
            </Stack>

            {/* Deploy */}
            <Stack direction="row" spacing={1} alignItems="center" mt={3}>
              <Button
                variant="contained"
                fullWidth
                disabled={selectedGatewayIds.length === 0 || deploying}
                onClick={handleDeploy}
              >
                {deploying ? "Deploying…" : "Deploy"}
              </Button>
            </Stack>
          </Card>
        </Grid>

        {/* RIGHT: one Development card per deployed gateway */}
        {Object.keys(deployByGateway).length === 0 ? (
          <Grid>
            <Card>
              <Typography fontWeight={800}>Development</Typography>
              <Divider sx={{ my: 2 }} />
              <Typography color="text.secondary">
                Select one or more gateways on the left and click{" "}
                <strong>Deploy</strong> to see status cards here.
              </Typography>
            </Card>
          </Grid>
        ) : (
          Object.entries(deployByGateway).map(([gatewayId, item]) => {
            const gw = gatewaysById.get(gatewayId);
            const title = gw ? gw.displayName || gw.name : "Gateway";
            const when = item.successDeployedTime || item.deployedTime || null;
            const status = (item.status || "").toString().toUpperCase() as
              | "ACTIVE"
              | "CREATED"
              | "FAILED"
              | "IN_PROGRESS"
              | "ROLLED_BACK"
              | "UNKNOWN";

            const success =
              status === "ACTIVE" ||
              status === "CREATED" ||
              status === "IN_PROGRESS";

            return (
              <Grid key={gatewayId}>
                <Card>
                  <Stack
                    direction="row"
                    justifyContent="space-between"
                    alignItems="center"
                    minWidth={300}
                  >
                    <Typography fontWeight={800}>{title}</Typography>
                    <Button
                      size="small"
                      variant="outlined"
                      color="error"
                      startIcon={<StopOutlinedIcon />}
                    >
                      Stop
                    </Button>
                  </Stack>

                  <Divider sx={{ my: 2 }} />

                  {/* Deployed time */}
                  <Stack
                    direction="row"
                    spacing={1}
                    alignItems="center"
                    sx={{ mb: 2 }}
                  >
                    <Typography>Deployed</Typography>
                    <AccessTimeIcon fontSize="small" sx={{ opacity: 0.7 }} />
                    <Typography color="text.secondary">
                      {when ? relativeTime(when) : "—"}
                    </Typography>
                  </Stack>

                  {/* Optional: show vhost used */}
                  {item.vhost && (
                    <Typography color="text.secondary">
                      vhost: {item.vhost}
                    </Typography>
                  )}

                  {/* Deployment Status banner */}
                  <Box
                  mt={3}
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
                      mb: 3,
                      display: "flex",
                      alignItems: "center",
                      justifyContent: "space-between",
                    }}
                  >
                    <Typography fontWeight={500}>Deployment Status</Typography>
                    <Chip
                      label={status || "—"}
                      color={success ? "success" : "error"}
                      variant={success ? "filled" : "outlined"}
                      size="small"
                    />
                  </Box>
                </Card>
              </Grid>
            );
          })
        )}
      </Grid>
    </Box>
  );
};

/* Wrap with ALL providers so APIs, Gateways and Deployments are available */
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
