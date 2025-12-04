import React from "react";
import {
  Box,
  CircularProgress,
  Grid,
  Stack,
  Typography,
  Button as MuiButton,
} from "@mui/material";
import { useSearchParams } from "react-router-dom";
import { useApisContext } from "../../context/ApiContext";
import type { ApiSummary, ApiGatewaySummary } from "../../hooks/apis";

import {
  DeploymentProvider,
  useDeployment,
} from "../../context/DeploymentContext";
import type { DeployRevisionResponseItem } from "../../hooks/deployments";

import { GatewayProvider, useGateways } from "../../context/GatewayContext";
import type { Gateway } from "../../hooks/gateways";
import GatewayDeployCard from "./ApiDeploy/GatewayDeployCard";
import GatewayPickTable from "./ApiDeploy/GatewayPickTable";
import { Card, CardActionArea } from "../../components/src/components/Card";
import { Button } from "../../components/src/components/Button";
import { SearchBar } from "../../components/src/components/SearchBar";
import { Tooltip } from "../../components/src/components/Tooltip";
import CardsPageLayout from "../../common/CardsPageLayout";

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

type Mode = "empty" | "pick" | "cards";

/* ---------------- page content ---------------- */

const DevelopContent: React.FC = () => {
  const { fetchApiById, fetchGatewaysForApi, addGatewaysToApi, selectApi, currentApi } =
    useApisContext();
  const { gateways, loading: gatewaysLoading } = useGateways();
  const { deployApiRevision, loading: deploying } = useDeployment();

  const [searchParams, setSearchParams] = useSearchParams();
  const searchParamsKey = searchParams.toString();
  const apiIdFromQuery = searchParams.get("apiId");
  const revisionIdFromQuery = searchParams.get("revisionId") ?? "7";
  const [query, setQuery] = React.useState("");

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

  // UI mode + selection
  const [mode, setMode] = React.useState<Mode>("empty");
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(
    () => new Set()
  );
  const [stagedIds, setStagedIds] = React.useState<string[]>([]);

  const effectiveApiId = React.useMemo(() => {
    if (apiIdFromQuery) return apiIdFromQuery;
    return currentApi?.id ?? "";
  }, [apiIdFromQuery, currentApi?.id]);

  React.useEffect(() => {
    if (apiIdFromQuery || !currentApi?.id) return;
    const next = new URLSearchParams(searchParamsKey);
    next.set("apiId", currentApi.id);
    setSearchParams(next, { replace: true });
  }, [apiIdFromQuery, currentApi?.id, searchParamsKey, setSearchParams]);

  React.useEffect(() => {
    let mounted = true;
    (async () => {
      try {
        if (!effectiveApiId) {
          if (mounted) {
            setLoading(false);
            setApi(null);
            setDeployedForApi([]);
            setDeployByGateway({});
            setStagedIds([]);
            setSelectedIds(new Set<string>());
            setMode("empty");
          }
          return;
        }

        setLoading(true);
        const [apiData, apiAssociatedGws] = await Promise.all([
          fetchApiById(effectiveApiId),
          fetchGatewaysForApi(effectiveApiId),
        ]);

        if (!mounted) return;

        setApi(apiData);
        selectApi(apiData);
        const onlyDeployed = apiAssociatedGws.filter((gw) => gw.isDeployed === true);
        setDeployedForApi(onlyDeployed);

        // Seed deployed gateways so they initially show the status section
        const nowIso = new Date().toISOString();
        const seeded: Record<string, DeployRevisionResponseItem> = {};
        onlyDeployed.forEach((gw) => {
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

        // Initial mode based on whether API has any associated gateways
        if (apiAssociatedGws.length > 0) {
          // Show Cards view with all associated gateways (both deployed and not)
          setStagedIds(apiAssociatedGws.map((g) => g.id));
          setMode("cards");
        } else {
          // No associations yet → show the Add Gateways tile
          setMode("empty");
        }
      } finally {
        if (mounted) setLoading(false);
      }
    })();
    return () => {
      mounted = false;
    };
  }, [
    effectiveApiId,
    fetchApiById,
    fetchGatewaysForApi,
    revisionIdFromQuery,
    selectApi,
  ]);

  const deployedMap = React.useMemo(() => {
    const m = new Map<string, ApiGatewaySummary>();
    deployedForApi.forEach((g) => m.set(g.id, g));
    return m;
  }, [deployedForApi]);

  const associatedIds = React.useMemo(
    () => new Set(stagedIds),
    [stagedIds]
  );

  const visibleGateways = React.useMemo(
    () => gateways.filter((g) => !associatedIds.has(g.id)),
    [gateways, associatedIds]
  );

  const gatewaysById = React.useMemo(() => {
    const m = new Map<string, Gateway>();
    gateways.forEach((g) => m.set(g.id, g));
    return m;
  }, [gateways]);

  const noMoreGateways = visibleGateways.length === 0;

  const handleDeploySingle = async (gatewayId: string) => {
    if (!effectiveApiId) return;
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
      const resp = await deployApiRevision(
        effectiveApiId,
        revisionIdFromQuery,
        targets
      );
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
            isCritical: (gw as any).isCritical ?? (gw as any).critical ?? false,
            functionalityType: (gw as any).functionalityType ?? "regular",
            isActive: true,
            createdAt: (gw as any).createdAt ?? undefined,
            updatedAt: new Date().toISOString(),
          };
          setDeployedForApi((prev) => [...prev, newly]);
          // optional: if this gateway was selected in pick mode, remove from selected
          setSelectedIds((prev) => {
            const next = new Set(prev);
            next.delete(gatewayId);
            return next;
          });
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

  // ---------- Selection handlers for table (operate on visible gateways only) ----------
  const toggleRow = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const [search, setSearch] = React.useState("");

  const areAllSelected =
    visibleGateways.length > 0 &&
    visibleGateways.every((g) => selectedIds.has(g.id));

  const isSomeSelected =
    selectedIds.size > 0 &&
    !areAllSelected &&
    visibleGateways.some((g) => selectedIds.has(g.id));

  const toggleAll = () => {
    if (areAllSelected) {
      // unselect only the visible ones
      setSelectedIds((prev) => {
        const next = new Set(prev);
        visibleGateways.forEach((g) => next.delete(g.id));
        return next;
      });
    } else {
      // select all visible
      setSelectedIds((prev) => {
        const next = new Set(prev);
        visibleGateways.forEach((g) => next.add(g.id));
        return next;
      });
    }
  };

  const clearSelection = () => setSelectedIds(new Set());

  const addSelection = async () => {
    if (selectedIds.size === 0) return;
    
    if (effectiveApiId) {
      try {
        const gatewayIdsToAdd = Array.from(selectedIds);
        const updatedGateways = await addGatewaysToApi(effectiveApiId, gatewayIdsToAdd);
        
        const onlyDeployed = updatedGateways.filter((gw) => gw.isDeployed === true);
        setDeployedForApi(onlyDeployed);
        
        setStagedIds((prev) => {
          const allGatewayIds = updatedGateways.map((gw) => gw.id);
          const combined = new Set([...prev, ...allGatewayIds]);
          return Array.from(combined);
        });
        
        setSelectedIds(new Set());
      } catch (error) {
        console.error("Failed to add gateways to API:", error);
        return;
      }
    }
    
    setMode("cards");
  };

  // ---------- Rendering helpers ----------
const renderEmptyTile = () => (
  <Grid container spacing={3}>
    <Grid >
      <Card
        testId={""}
        style={{
          width: 300,
          height: 200,
          border: "1px solid #4caf50",
          borderColor: "success.light",
        }}
      >
        <CardActionArea
          onClick={() => setMode("pick")}
          testId={""}
          sx={{
            height: "100%",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Stack alignItems="center" spacing={0.5}>
            <Typography variant="h4" fontWeight={700}>
              +
            </Typography>
            <Typography variant="h4" fontWeight={700}>
              Add Gateways
            </Typography>
            <Typography color="text.secondary">
              Select gateways you need to Deploy
            </Typography>
          </Stack>
        </CardActionArea>
      </Card>
    </Grid>
  </Grid>
);


  const renderCards = () => {
    const selectedGateways = stagedIds
      .map((id) => gatewaysById.get(id))
      .filter((g): g is Gateway => Boolean(g));

    // filter by SearchBar query
    const q = query.trim().toLowerCase();
    const filteredGateways = q
      ? selectedGateways.filter((gw) => {
          const text =
            `${gw.displayName || gw.name || ""} ` +
            `${gw.description || ""} ` +
            `${gw.vhost || ""}`;
          return text.toLowerCase().includes(q);
        })
      : selectedGateways;

    return selectedGateways.length === 0 ? (
      <Card style={{ marginTop: 2 }} testId={""}>
        <Typography color="text.secondary">
          No gateways selected. Click “Add Gateways” to choose.
        </Typography>
      </Card>
    ) : (
      <CardsPageLayout
        cardWidth={380}
        topLeft={<Typography variant="h3">Selected Gateways</Typography>}
        topRight={
          <Stack direction="row" spacing={1} alignItems="center">
            <SearchBar
              testId="selected-gateways-search"
              placeholder="Search selected gateways"
              inputValue={query}
              onChange={setQuery}
              iconPlacement="left"
              bordered
              size="medium"
              color="secondary"
            />
            <Tooltip
              title={
                noMoreGateways
                  ? "All available gateways are already added"
                  : ""
              }
              placement="bottom"
              arrow
              disableHoverListener={!noMoreGateways}
              disableFocusListener={!noMoreGateways}
              disableTouchListener={!noMoreGateways}
            >
              <span>
                <Button
                  onClick={() => setMode("pick")}
                  disabled={noMoreGateways}
                >
                  Add Gateways
                </Button>
              </span>
            </Tooltip>
          </Stack>
        }
      >
        {filteredGateways.map((gw) => (
          <GatewayDeployCard
            key={gw.id}
            gw={gw}
            apiId={effectiveApiId}
            api={api}
            deployedMap={deployedMap}
            deployByGateway={deployByGateway}
            deploying={deploying}
            deployingIds={deployingIds}
            relativeTime={relativeTime}
            onDeploy={handleDeploySingle}
          />
        ))}
      </CardsPageLayout>
    );
  };

  if (!effectiveApiId) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700}>
          Select an API to deploy.
        </Typography>
        <Typography color="text.secondary">
          Choose an API from the header picker to manage deployments.
        </Typography>
      </Box>
    );
  }

  if (loading || gatewaysLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box>
      {/* <Box mb={2}>
        <Typography variant="h3" fontWeight={700}>
          Deploy
        </Typography>
      </Box> */}

      {mode === "empty" && renderEmptyTile()}

      {mode === "pick" && (
        <GatewayPickTable
          gateways={visibleGateways}
          selectedIds={selectedIds}
          areAllSelected={areAllSelected}
          isSomeSelected={isSomeSelected}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onClear={clearSelection}
          onAdd={addSelection}
          relativeTime={relativeTime}
          onBack={() => setMode(stagedIds.length ? "cards" : "empty")}
        />
      )}

      {mode === "cards" && renderCards()}
    </Box>
  );
};

/* Wrap with providers */
const ApiDevelop: React.FC = () => (
  <GatewayProvider>
    <DeploymentProvider>
      <DevelopContent />
    </DeploymentProvider>
  </GatewayProvider>
);

export default ApiDevelop;
