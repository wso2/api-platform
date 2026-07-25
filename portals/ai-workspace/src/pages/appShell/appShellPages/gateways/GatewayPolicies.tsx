/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import {
  Alert,
  Button,
  Chip,
  CircularProgress,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { useGatewayPolicies } from "../../../../contexts/GatewayPoliciesContext";
import useAIWorkspaceSnackbar from "../../../../hooks/aiWorkspaceSnackbar";

export default function GatewayPolicies() {
  const { policies, isLoading, error, refresh, syncPolicy, syncingPolicyKey } =
    useGatewayPolicies();
  const showSnackbar = useAIWorkspaceSnackbar();

  const handleSync = async (policyName: string, version: string) => {
    try {
      await syncPolicy(policyName, version);
      showSnackbar(
        `Policy ${policyName} version ${version} synced successfully.`,
        "success",
      );
    } catch (cause) {
      const message =
        cause instanceof Error
          ? cause.message
          : "Failed to sync the custom policy.";
      showSnackbar(message, "error");
    }
  };

  if (isLoading) {
    return (
      <TableContainer sx={{ border: 1, borderColor: "divider", px: 3, py: 2 }}>
        <Table>
          <TableHead>
            <TableRow>
              <TableCell sx={{ width: 220 }}>Name</TableCell>
              <TableCell sx={{ width: 112 }}>Version</TableCell>
              <TableCell>Description</TableCell>
              <TableCell sx={{ width: 190 }}>Policy Type</TableCell>
              <TableCell sx={{ width: 120 }}>Sync Status</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {[0, 1, 2, 3].map((key) => (
              <TableRow key={key}>
                <TableCell>
                  <Skeleton variant="text" width="70%" height={20} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="rounded" width={64} height={24} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="text" width="90%" height={20} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="rounded" width={140} height={24} />
                </TableCell>
                <TableCell>
                  <Skeleton variant="rounded" width={100} height={24} />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </TableContainer>
    );
  }
  if (error) {
    return (
      <Alert
        severity="error"
        action={
          <Button color="inherit" size="small" onClick={() => void refresh()}>
            Retry
          </Button>
        }
      >
        {error.message}
      </Alert>
    );
  }
  if (!policies.length)
    return <Alert severity="info">No gateway manifest received yet.</Alert>;

  return (
    <TableContainer sx={{ border: 1, borderColor: "divider", px: 3, py: 2 }}>
      <Table>
        <TableHead>
          <TableRow>
            <TableCell sx={{ width: 220 }}>Name</TableCell>
            <TableCell sx={{ width: 112 }}>Version</TableCell>
            <TableCell>Description</TableCell>
            <TableCell sx={{ width: 190 }}>Policy Type</TableCell>
            <TableCell sx={{ width: 120 }}>Sync Status</TableCell>
          </TableRow>
        </TableHead>
        <TableBody>
          {policies.map((policy) => (
            <TableRow key={policy.key} hover>
              <TableCell>
                <Typography variant="body2">{policy.name}</Typography>
              </TableCell>
              <TableCell>
                <Chip
                  label={`v${policy.displayVersion}`}
                  size="small"
                  variant="outlined"
                  sx={{ minWidth: 72 }}
                />
              </TableCell>
              <TableCell>
                <Typography variant="body2" color="text.secondary">
                  {policy.description}
                </Typography>
              </TableCell>
              <TableCell>
                <Chip
                  label={policy.policyType}
                  size="small"
                  variant="outlined"
                  sx={{ minWidth: 160 }}
                />
              </TableCell>
              <TableCell>
                {policy.syncStatus === "N/A" ? (
                  <Typography variant="body2" color="text.secondary">
                    N/A
                  </Typography>
                ) : policy.syncStatus === "Synced" ? (
                  <Chip
                    label="Latest Version Available"
                    size="small"
                    color="success"
                    variant="outlined"
                  />
                ) : (
                  <Tooltip title="Click to sync the policy">
                    <span>
                      <Button
                        size="small"
                        variant="contained"
                        disabled={syncingPolicyKey !== null}
                        onClick={() =>
                          void handleSync(policy.policyName, policy.version)
                        }
                        startIcon={
                          syncingPolicyKey === policy.key ? (
                            <CircularProgress size={14} color="inherit" />
                          ) : undefined
                        }
                      >
                        Sync
                      </Button>
                    </span>
                  </Tooltip>
                )}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </TableContainer>
  );
}
