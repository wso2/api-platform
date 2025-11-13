import * as React from "react";
import { Box, Stack, Typography } from "@mui/material";
import { Chip } from "../Chip";

/** ---------- Shared Types ---------- */
// For UI rendering (name can be missing in validation results)
export type DisplayOperation = {
  name?: string;
  description?: string;
  request: {
    method: string;
    path: string;
    ["backend-services"]?: Array<{ name: string }>;
  };
};

// For API payloads (name MUST be present)
export type ContractOperation = {
  name: string;
  description?: string;
  request: {
    method: string;
    path: string;
    ["backend-services"]?: Array<{ name: string }>;
  };
};

export type OpenAPI = {
  openapi?: string;
  swagger?: string;
  info?: { title?: string; version?: string; description?: string };
  servers?: { url?: string }[];
  paths?: Record<
    string,
    Record<
      string,
      { summary?: string; description?: string; operationId?: string }
    >
  >;
};

const METHOD_COLORS: Record<
  string,
  "primary" | "success" | "warning" | "error" | "info" | "secondary"
> = {
  GET: "info",
  POST: "success",
  PUT: "warning",
  DELETE: "error",
  PATCH: "secondary",
  HEAD: "primary",
  OPTIONS: "primary",
};

const METHOD_ORDER = ["get", "post", "put", "delete", "patch", "head", "options"] as const;

/** ---------- Reusable Operations List (for UI) ---------- */
export const ApiOperationsList: React.FC<{
  title?: string;
  operations?: DisplayOperation[];
  maxHeight?: number;
}> = ({ title = "API Operations", operations, maxHeight = 500 }) => {
  if (!operations?.length) return null;

  return (
    <Box sx={{ border: "1px solid", borderColor: "divider", borderRadius: 2, overflow: "hidden", bgcolor: "background.paper" }}>
      <Box sx={{ px: 2, py: 1.25, borderBottom: "1px solid", borderColor: "divider", fontWeight: 700 }}>
        {title}
      </Box>

      <Box sx={{ maxHeight, overflow: "auto" }}>
        {operations.map((op, idx) => {
          const method = (op?.request?.method || "").toUpperCase();
          const color = METHOD_COLORS[method] ?? "primary";
          return (
            <Box
              key={`${op?.name || op?.request?.path || idx}-${idx}`}
              sx={{
                px: 2,
                py: 1.5,
                display: "grid",
                gridTemplateColumns: "120px 1fr",
                alignItems: "center",
                borderBottom: idx === operations.length - 1 ? "none" : "1px solid",
                borderColor: "divider",
              }}
            >
              <Chip
                size="large"
                color={color}
                label={method || "-"}
                sx={{ fontWeight: 700, width: 72, justifySelf: "start" }}
              />
              <Stack direction="row" spacing={1.5} alignItems="center">
                <Typography variant="body2" sx={{ fontFamily: "monospace" }}>
                  {op?.request?.path || "/"}
                </Typography>
                <Typography variant="body2" color="#7c7c7cff" noWrap>
                  {op?.name || op?.description || ""}
                </Typography>
              </Stack>
            </Box>
          );
        })}
      </Box>
    </Box>
  );
};

/** ---------- Helper: Build contract-safe operations from OpenAPI ---------- */
export function buildOperationsFromOpenAPI(
  doc?: OpenAPI,
  serviceName?: string
): ContractOperation[] {
  if (!doc?.paths) return [];

  const ops: ContractOperation[] = [];
  const serviceRef = serviceName ? [{ name: serviceName }] : [];

  const sortedPaths = Object.keys(doc.paths).sort((a, b) => a.localeCompare(b));
  for (const path of sortedPaths) {
    const methods = doc.paths[path] || {};
    for (const m of METHOD_ORDER) {
      const op = (methods as any)[m];
      if (!op) continue;

      const opId = (op.operationId as string | undefined)?.trim();
      const pretty =
        opId ||
        `${m.toUpperCase()} ${path}`.replace(/[{}]/g, "").replace(/\s+/g, "_");

      ops.push({
        name: pretty, // guaranteed
        description: op.summary || op.description || undefined,
        request: {
          method: m.toUpperCase(),
          path,
          "backend-services": serviceRef.length ? serviceRef : undefined,
        },
      });
    }
  }
  return ops;
}
