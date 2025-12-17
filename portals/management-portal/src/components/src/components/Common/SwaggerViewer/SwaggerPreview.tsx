import React, { Suspense, useEffect, useMemo, useState } from "react";
import { Box, Typography, CircularProgress } from "@mui/material";
import SwaggerUI from "swagger-ui-react";
import "swagger-ui-react/swagger-ui.css";
import "./swaggerPreview.css";

const MAX_RESPONSE_SIZE = 4 * 1024 * 1024;

type Props = {
  title?: string;
  definitionUrl?: string | null;
  definitionFile?: File | null;
  isValid?: boolean;
  isLoading?: boolean;
  placeholder?: string;
  maxHeight?: number;
  minHeight?: number;
  apiEndpoint?: string;
  token?: string;
  headerName?: string;
  testSessionHeaderName?: string;
  testSessionId?: string;
  docExpansion?: "list" | "full" | "none";
};

type AnyObj = Record<string, any>;

async function parseOpenApiFile(file: File): Promise<AnyObj> {
  const text = await file.text();
  try {
    return JSON.parse(text);
  } catch {
    const yaml = await import("js-yaml");
    const doc = yaml.load(text);
    if (!doc || typeof doc !== "object") {
      throw new Error("Invalid OpenAPI file (not a JSON/YAML object).");
    }
    return doc as AnyObj;
  }
}

function cloneDeep<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

function normalizeApiEndpoint(endpoint: string) {
  const trimmed = endpoint.trim();
  return trimmed.endsWith("/") ? trimmed.slice(0, -1) : trimmed;
}

function applyEndpointToSpec(spec: AnyObj, apiEndpoint?: string) {
  if (!spec || !apiEndpoint) return spec;

  const endpoint = normalizeApiEndpoint(apiEndpoint);
  const next = cloneDeep(spec);

  if (typeof next.swagger === "string" && next.swagger.startsWith("2")) {
    try {
      const u = new URL(endpoint);
      next.host = u.host;
      next.schemes = [u.protocol.replace(":", "")];
      next.basePath = u.pathname && u.pathname !== "/" ? u.pathname : "/";
    } catch {
      next.host = endpoint.replace(/^https?:\/\//, "");
      next.basePath = endpoint.endsWith("/") ? "" : "/";
    }
    return next;
  }

  if (typeof next.openapi === "string" && next.openapi.startsWith("3")) {
    next.servers = [{ url: endpoint }];
    return next;
  }

  return next;
}

export const SwaggerPreview: React.FC<Props> = ({
  definitionUrl,
  definitionFile,
  isValid = false,
  isLoading = false,
  placeholder = "Upload and validate an OpenAPI definition to see a preview.",
  maxHeight = 600,
  minHeight = 500,
  apiEndpoint,
  token,
  headerName,
  testSessionHeaderName,
  testSessionId,
  docExpansion = "list",
}) => {
  const [fileSpec, setFileSpec] = useState<AnyObj | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const [fileParsing, setFileParsing] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function run() {
      if (!definitionFile) {
        setFileSpec(null);
        setFileError(null);
        setFileParsing(false);
        return;
      }

      setFileParsing(true);
      setFileError(null);

      try {
        const parsed = await parseOpenApiFile(definitionFile);
        if (!cancelled) setFileSpec(parsed);
      } catch (e: any) {
        if (!cancelled) {
          setFileSpec(null);
          setFileError(e?.message || "Failed to parse OpenAPI file.");
        }
      } finally {
        if (!cancelled) setFileParsing(false);
      }
    }

    run();
    return () => {
      cancelled = true;
    };
  }, [definitionFile]);

  const resolvedUrl = useMemo(() => {
    const trimmed = (definitionUrl || "").trim();
    return trimmed || null;
  }, [definitionUrl]);

  const resolvedSpec = useMemo(() => {
    if (fileSpec) return applyEndpointToSpec(fileSpec, apiEndpoint);
    return null;
  }, [fileSpec, apiEndpoint]);

  const requestInterceptor = (req: any) => {
    if (token) {
      if (headerName) {
        req.headers[headerName] = token;
      } else {
        req.headers["Authorization"] = token.startsWith("Bearer ")
          ? token
          : `Bearer ${token}`;
      }
    }

    if (testSessionHeaderName && testSessionId) {
      req.headers[testSessionHeaderName] = testSessionId;
    }

    if (typeof req.url === "string") {
      req.url = req.url.replace("/*", "");
    }

    return req;
  };

  const responseInterceptor = (response: any) => {
    if (response?.status === 200) {
      const payload = response.data;
      const size = new Blob([payload]).size;
      if (size > MAX_RESPONSE_SIZE) {
        return { ...response, text: "Response is too large to render" };
      }
    }
    return response;
  };

  const hideSchemesAndAuthorizePlugin = {
    statePlugins: {
      spec: {
        wrapSelectors: {
          servers: () => () => [],
          securityDefinitions: () => () => null,
          schemes: () => () => [],
        },
      },
    },
    wrapComponents: {
      authorizeBtn: () => () => null,
      info: () => () => null,
    },
  };

  const shouldShowSwagger =
    !isLoading &&
    isValid &&
    (Boolean(resolvedSpec) || Boolean(resolvedUrl)) &&
    !fileParsing &&
    !fileError;

  return (
    <Box
      className="swaggerPreviewRoot"
      sx={{
        flex: 1,
        maxHeight,
        minHeight,
        overflow: "auto",
        bgcolor: "background.paper",
        padding: 2,
      }}
    >
      {isLoading || fileParsing ? (
        <Box className="swaggerPreviewLoader">
          <CircularProgress size={30} />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 1 }}>
            {isLoading
              ? "Validating OpenAPI definition..."
              : "Loading preview..."}
          </Typography>
        </Box>
      ) : fileError ? (
        <Typography variant="body2" color="error" sx={{ p: 2 }}>
          {fileError}
        </Typography>
      ) : !shouldShowSwagger ? (
        <Typography variant="body2" color="text.secondary" sx={{ p: 2 }}>
          {placeholder}
        </Typography>
      ) : (
        <Suspense
          fallback={
            <Box className="swaggerPreviewLoader">
              <CircularProgress size={30} />
            </Box>
          }
        >
          <SwaggerUI
            spec={resolvedSpec || undefined}
            url={!resolvedSpec ? resolvedUrl || undefined : undefined}
            requestInterceptor={requestInterceptor}
            responseInterceptor={responseInterceptor}
            plugins={[hideSchemesAndAuthorizePlugin]}
            defaultModelExpandDepth={-1}
            defaultModelsExpandDepth={0}
            docExpansion={docExpansion}
            supportedSubmitMethods={[]}
          />
        </Suspense>
      )}
    </Box>
  );
};
