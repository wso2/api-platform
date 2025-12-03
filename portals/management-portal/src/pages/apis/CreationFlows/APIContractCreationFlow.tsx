import * as React from "react";
import { Box, Stack, Typography, Tabs, Tab } from "@mui/material";
import ArrowLeftLong from "../../../components/src/Icons/generated/ArrowLeftLong";
import GithubCreationFlow from "./ContractCreationFlows/GithubCreationFlow";
import UploadCreationFlow from "./ContractCreationFlows/UploadCreationFlow";
import URLCreationFlow from "./ContractCreationFlows/URLCreationFlow";
import { Button } from "../../../components/src/components/Button";
import type { ImportOpenApiRequest, ApiSummary } from "../../../hooks/apis";

type Props = {
  open: boolean;
  selectedProjectId: string;
  importOpenApi: (payload: ImportOpenApiRequest, opts?: { signal?: AbortSignal }) => Promise<ApiSummary>;
  refreshApis: (projectId?: string) => Promise<ApiSummary[]>;
  onClose: () => void;
};

type TabKey = "upload" | "github" | "url";

const APIContractCreationFlow: React.FC<Props> = ({
  open,
  selectedProjectId,
  importOpenApi,
  refreshApis,
  onClose,
}) => {
  const [tab, setTab] = React.useState<TabKey>("upload");

  const tabSx = {
    textTransform: "capitalize",
    color: "#2d2d2dff",
    letterSpacing: 0.25,
    "&.Mui-selected": {
      color: "#2d2d2dff",
      fontWeight: 600,
    },
  };

  React.useEffect(() => {
    if (open) setTab("upload");
  }, [open]);

  if (!open) return null;

  return (
    <Box>
      <Box mb={1}>
        <Button
          onClick={onClose}
          variant="link"
          startIcon={<ArrowLeftLong fontSize="small" />}
        >
          Back to Home
        </Button>
      </Box>

      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{ mb: 1 }}
      >
        <Typography variant="h3" fontWeight={600}>
          Import API Contract
        </Typography>
      </Stack>

      {/* Tabs with bottom border */}
      <Box sx={{ borderBottom: "1px solid", borderColor: "divider", mb: 3 }}>
        <Tabs
          value={tab}
          onChange={(_e, v) => setTab(v as TabKey)}
        >
          <Tab label="Upload" value="upload" sx={tabSx} />
          <Tab label="GitHub" value="github" sx={tabSx} />
          <Tab label="URL" value="url" sx={tabSx} />
        </Tabs>
      </Box>

      {/* Child flows */}
      {tab === "upload" && (
        <UploadCreationFlow
          open={open}
          selectedProjectId={selectedProjectId}
          importOpenApi={importOpenApi}
          refreshApis={refreshApis}
          onClose={onClose}
        />
      )}
      {tab === "github" && (
        <GithubCreationFlow
          open={open}
          selectedProjectId={selectedProjectId}
          // createApi={createApi}
          refreshApis={refreshApis}
          onClose={onClose}
        />
      )}
      {tab === "url" && (
        <URLCreationFlow
          open={open}
          selectedProjectId={selectedProjectId}
          importOpenApi={importOpenApi}
          refreshApis={refreshApis}
          onClose={onClose}
        />
      )}
    </Box>
  );
};

export default APIContractCreationFlow;
