import React from "react";
import {
  Box,
  CircularProgress,
  Grid,
  Stack,
  Typography,
  Alert,
} from "@mui/material";
import { useSearchParams } from "react-router-dom";
import { useApisContext } from "../../context/ApiContext";
import type { ApiSummary } from "../../hooks/apis";

import { DevPortalProvider, useDevPortals } from "../../context/DevPortalContext";
import type { Portal } from "../../hooks/devportals";
import { ApiPublishProvider, useApiPublishing} from "../../context/ApiPublishContext";
import { Card, CardActionArea } from "../../components/src/components/Card";
import { Button } from "../../components/src/components/Button";
import { SearchBar } from "../../components/src/components/SearchBar";
import DevPortalDeployCard from "./ApiPublish/DevPortalDeployCard";
import DevPortalPickTable from "./ApiPublish/DevPortalPickTable";
import ApiPublishModal from "./ApiPublish/ApiPublishModal";
import type { ApiPublicationWithPortal } from "../../hooks/apiPublish";
import { relativeTime } from "../overview/utils";

type Mode = "empty" | "pick" | "cards";

/* ---------------- page content ---------------- */

const DevelopContent: React.FC = () => {
  const { fetchApiById, selectApi } = useApisContext();
  const { devportals, refreshDevPortals, loading: devportalsLoading } = useDevPortals();
  const { publishedApis, loading: publishingLoading, refreshPublishedApis, publishApiToDevPortal } = useApiPublishing();

  const [searchParams, setSearchParams] = useSearchParams();
  const searchParamsKey = searchParams.toString();
  const apiIdFromQuery = searchParams.get("apiId");

  const [api, setApi] = React.useState<ApiSummary | null>(null);
  const [loading, setLoading] = React.useState(true);

  // Local published state
  // const [publishedForApi, setPublishedForApi] = React.useState<ApiPublicationWithPortal[]>([]);

  // UI mode + selection
  const [mode, setMode] = React.useState<Mode>("empty");
  const [selectedIds, setSelectedIds] = React.useState<Set<string>>(() => new Set());
  const [stagedIds, setStagedIds] = React.useState<string[]>([]);
  const [query, setQuery] = React.useState("");

  // Publishing state
  const [publishingIds, setPublishingIds] = React.useState<Set<string>>(() => new Set());

  // Modal state
  const [publishModalOpen, setPublishModalOpen] = React.useState(false);
  const [selectedPortalForPublish, setSelectedPortalForPublish] = React.useState<ApiPublicationWithPortal | null>(null);

  const effectiveApiId = React.useMemo(() => {
    if (apiIdFromQuery) return apiIdFromQuery;
    return ""; // No current API fallback
  }, [apiIdFromQuery]);

  React.useEffect(() => {
    if (apiIdFromQuery || !api) return;
    const next = new URLSearchParams(searchParamsKey);
    next.set("apiId", api.id);
    setSearchParams(next, { replace: true });
  }, [apiIdFromQuery, api?.id, searchParamsKey, setSearchParams]);

  React.useEffect(() => {
    
    if (!effectiveApiId) {
      setLoading(false);
      setApi(null);
      return;
    }

    (async () => {
      setLoading(true);
      try {
        const apiData = await fetchApiById(effectiveApiId);
        setApi(apiData);
        selectApi(apiData);

        // Refresh devportals and published APIs
        await Promise.all([
          refreshDevPortals(),
          refreshPublishedApis(effectiveApiId).then((pubs) => {
            if (pubs.length > 0) {
              setStagedIds(pubs.map(p => p.uuid));
              setMode("cards");
            } else {
              setMode("empty");
            }
          })
        ]);
      } catch (error) {
        console.error('Failed to load API data:', error);
        setApi(null);
        setMode("empty");
      } finally {
        setLoading(false);
      }
    })();
  }, [effectiveApiId, fetchApiById, selectApi, refreshDevPortals, refreshPublishedApis]);

  // Get published APIs for current API
  const apiPublished = publishedApis[effectiveApiId] || [];

  // Create published map
  const publishedMap = React.useMemo(() => {
    const m = new Map<string, ApiPublicationWithPortal>();
    apiPublished.forEach((publish) => {
      m.set(publish.uuid, publish);
    });
    return m;
  }, [apiPublished]);

  const devportalsById = React.useMemo(() => {
    const m = new Map<string, Portal>();
    devportals.forEach((p) => m.set(p.uuid, p));
    return m;
  }, [devportals]);

  const handlePublishToDevPortal = (portal: ApiPublicationWithPortal) => {
    setSelectedPortalForPublish(portal);
    setPublishModalOpen(true);
  };

  const handlePublishFromModal = async (portalId: string, payload: any) => {
    if (!effectiveApiId || !api) return;

    try {
      setPublishingIds((prev) => new Set(prev).add(portalId));
      await publishApiToDevPortal(effectiveApiId, payload);

      // No need to refresh here, context does it
    } finally {
      setPublishingIds((prev) => {
        const next = new Set(prev);
        next.delete(portalId);
        return next;
      });
    }
  };

  // Selection handlers for table
  const toggleRow = (id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const areAllSelected =
    devportals.length > 0 &&
    devportals.every((p) => selectedIds.has(p.uuid));

  const isSomeSelected =
    selectedIds.size > 0 &&
    !areAllSelected &&
    devportals.some((p) => selectedIds.has(p.uuid));

  const toggleAll = () => {
    if (areAllSelected) {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        devportals.forEach((p) => next.delete(p.uuid));
        return next;
      });
    } else {
      setSelectedIds((prev) => {
        const next = new Set(prev);
        devportals.forEach((p) => next.add(p.uuid));
        return next;
      });
    }
  };

  const clearSelection = () => setSelectedIds(new Set());

  const addSelection = () => {
    if (selectedIds.size === 0) return;
    // setStagedIds(Array.from(selectedIds));
    setStagedIds((prev) => [...new Set([...prev, ...Array.from(selectedIds)])]);
    setMode("cards");
  };

  // ---------- Rendering helpers ----------
  const renderEmptyTile = () => (
    <Grid container spacing={3}>
      <Grid>
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
                Select Dev Portals
              </Typography>
              <Typography color="text.secondary">
                Choose dev portals to publish your API
              </Typography>
            </Stack>
          </CardActionArea>
        </Card>
      </Grid>
    </Grid>
  );

  const renderCards = () => {
    const selectedPortals = stagedIds.map((id) => {
      const published = (publishedApis[effectiveApiId] || []).find(p => p.uuid === id);
      if (published) return published;
      const portal = devportalsById.get(id);
      if (portal) {
        // Properly construct ApiPublicationWithPortal from Portal
        return {
          uuid: portal.uuid,
          name: portal.name,
          identifier: portal.identifier,
          description: portal.description,
          portalUrl: portal.uiUrl || "",
          apiUrl: portal.apiUrl || "",
          hostname: portal.hostname || "",
          isActive: portal.isActive || false,
          createdAt: portal.createdAt || "",
          updatedAt: portal.updatedAt || "",
          associatedAt: "",
          isPublished: false,
          publication: {
            status: "UNPUBLISHED",
            apiVersion: "",
            sandboxEndpoint: "",
            productionEndpoint: "",
            publishedAt: "",
            updatedAt: "",
          },
        } as ApiPublicationWithPortal;
      }
      return null;
    }).filter((p): p is ApiPublicationWithPortal => Boolean(p));

    // filter by SearchBar query
    const q = query.trim().toLowerCase();
    const filteredPortals = q
      ? selectedPortals.filter((p) => {
          const text = `${p.name || ""} ${p.description || ""} ${p.portalUrl || ""}`;
          return text.toLowerCase().includes(q);
        })
      : selectedPortals;

    return selectedPortals.length === 0 ? (
      <Card style={{ marginTop: 2 }} testId={""}>
        <Typography color="text.secondary">
          No dev portals selected. Click "Select Dev Portals" to choose.
        </Typography>
      </Card>
    ) : (
      <>
        {/* Title at start; SearchBar + Button at end */}
        <Stack
          direction="row"
          justifyContent="space-between"
          alignItems="center"
          mb={3}
          gap={2}
        >
          <Typography variant="h4" fontWeight={600}>
            Selected Dev Portals
          </Typography>

          <Stack
            direction="row"
            alignItems="center"
            gap={1.5}
            flexShrink={0}
            sx={{
              minWidth: 500,
              maxWidth: 700,
              flex: 1,
              justifyContent: "flex-end",
            }}
          >
            <Box sx={{ flex: 1, maxWidth: 420 }}>
              <SearchBar
                testId="selected-devportals-search"
                placeholder="Search selected dev portals"
                inputValue={query}
                onChange={setQuery}
                iconPlacement="left"
                bordered
                size="medium"
                color="secondary"
              />
            </Box>
            <Button onClick={() => setMode("pick")}>
              Add Dev Portals
            </Button>
          </Stack>
        </Stack>

        {filteredPortals.length === 0 && selectedPortals.length > 0 && query ? (
          <Alert severity="info" sx={{ mb: 3 }}>
            No dev portals match "{query}". Try a different search term.
          </Alert>
        ) : null}

        <Grid container spacing={3}>
          {filteredPortals.map((portal) => (
            <DevPortalDeployCard
              key={portal.uuid}
              portal={portal}
              apiId={effectiveApiId}
              publishingIds={publishingIds}
              relativeTime={relativeTime}
              onPublish={handlePublishToDevPortal}
            />
          ))}
        </Grid>
      </>
    );
  };

  if (!effectiveApiId) {
    return (
      <Box textAlign="center" mt={6}>
        <Typography variant="h5" fontWeight={700}>
          Select an API to publish.
        </Typography>
        <Typography color="text.secondary">
          Choose an API from the header picker to manage publications.
        </Typography>
      </Box>
    );
  }

  if (loading || devportalsLoading) {
    return (
      <Box display="flex" alignItems="center" justifyContent="center" mt={6}>
        <CircularProgress size={32} />
      </Box>
    );
  }

  return (
    <Box>
      {mode === "empty" && renderEmptyTile()}

      {mode === "pick" && (
        <DevPortalPickTable
          portals={devportals}
          selectedIds={selectedIds}
          areAllSelected={areAllSelected}
          isSomeSelected={isSomeSelected}
          onToggleRow={toggleRow}
          onToggleAll={toggleAll}
          onClear={clearSelection}
          onAdd={addSelection}
          publishedIds={Array.from(publishedMap.keys())}
          onBack={() => setMode(stagedIds.length ? "cards" : "empty")}
        />
      )}

      {mode === "cards" && renderCards()}

      {/* Publish Modal */}
      {selectedPortalForPublish && (
        <ApiPublishModal
          open={publishModalOpen}
          portal={{
            uuid: selectedPortalForPublish.uuid,
            name: selectedPortalForPublish.name,
          }}
          api={api!}
          // gateways={gateways}
          onClose={() => {
            setPublishModalOpen(false);
            setSelectedPortalForPublish(null);
          }}
          onPublish={handlePublishFromModal}
          publishing={publishingIds.has(selectedPortalForPublish.uuid)}
        />
      )}
    </Box>
  );
};
const ApiPublish: React.FC = () => (
  <DevPortalProvider>
    <ApiPublishProvider>
      <DevelopContent />
    </ApiPublishProvider>
  </DevPortalProvider>
);

export default ApiPublish;