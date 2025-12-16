import * as React from "react";
import { Box, Paper, Stack, Typography, CircularProgress } from "@mui/material";
import yaml from "js-yaml";
import { SwaggerPreview } from "../Common/SwaggerViewer/SwaggerPreview";
import { Toggler } from "../Toggler";

type Props = {
  title?: string;
  definitionUrl?: string | null;
  definitionFile?: File | null;
  isValid?: boolean;
  isLoading?: boolean;
  placeholder?: string;
  maxHeight?: number;
  apiEndpoint?: string;
};

const isProbablyJson = (text: string) => {
  const t = text.trim();
  return t.startsWith("{") || t.startsWith("[");
};

const prettySource = (raw: string) => {
  const trimmed = raw.trim();
  if (!trimmed) return "";

  if (isProbablyJson(trimmed)) {
    try {
      const obj = JSON.parse(trimmed);
      return JSON.stringify(obj, null, 2);
    } catch {
      return raw;
    }
  }

  try {
    const obj = yaml.load(trimmed);
    if (!obj || typeof obj !== "object") return raw;
    return yaml.dump(obj, { indent: 2, lineWidth: 120, noRefs: true });
  } catch {
    return raw;
  }
};

export const SwaggerPreviewWithSource: React.FC<Props> = ({
  title = "Fetched OAS Definition",
  definitionUrl,
  definitionFile,
  isValid = false,
  isLoading = false,
  placeholder,
  maxHeight = 500,
  apiEndpoint,
}) => {
  const [showSource, setShowSource] = React.useState(false);
  const [raw, setRaw] = React.useState<string>("");
  const [loadingSource, setLoadingSource] = React.useState(false);
  const [sourceError, setSourceError] = React.useState<string | null>(null);

  const resolvedUrl = React.useMemo(
    () => (definitionUrl || "").trim() || null,
    [definitionUrl]
  );

  const loadSource = React.useCallback(async () => {
    setSourceError(null);
    setLoadingSource(true);

    try {
      if (definitionFile) {
        const text = await definitionFile.text();
        setRaw(prettySource(text));
        return;
      }

      if (resolvedUrl) {
        const res = await fetch(resolvedUrl);
        if (!res.ok) throw new Error(`Failed to fetch source (${res.status})`);
        const text = await res.text();
        setRaw(prettySource(text));
        return;
      }

      setRaw("");
    } catch (e: any) {
      setRaw("");
      setSourceError(e?.message || "Failed to load source");
    } finally {
      setLoadingSource(false);
    }
  }, [definitionFile, resolvedUrl]);

  React.useEffect(() => {
    if (!showSource) return;
    if (!isValid) return;
    if (isLoading) return;
    loadSource();
  }, [showSource, isValid, isLoading, loadSource]);

  React.useEffect(() => {
    setRaw("");
    setSourceError(null);
    setLoadingSource(false);
  }, [definitionFile, resolvedUrl]);

  return (
    <Paper
      variant="outlined"
      sx={{
        borderRadius: 2,
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        maxWidth: "800px",
        backgroundColor: "#F8F8F8",
      }}
    >
      <Stack
        direction="row"
        alignItems="center"
        justifyContent="space-between"
        sx={{
          px: 2,
          py: 1.25,
          borderBottom: "1px solid",
          borderColor: "divider",
        }}
      >
        <Typography>{title}</Typography>

        <Stack direction="row" alignItems="center" spacing={1}>
          <Typography variant="body1">Source</Typography>
          <Toggler
            size="medium"
            checked={showSource}
            onClick={() => setShowSource((prev) => !prev)}
            color="primary"
            testId="swagger-source-toggle"
          />
        </Stack>
      </Stack>

      <Box sx={{ flex: 1, minHeight: 500, overflow: "auto", maxHeight: 500 }}>
        {showSource ? (
          <Box sx={{ p: 2, minHeight: 500 }}>
            {isLoading || loadingSource ? (
              <Stack alignItems="center" justifyContent="center" sx={{ py: 6, }}>
                <CircularProgress size={28} />
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ mt: 1 }}
                >
                  Loading source...
                </Typography>
              </Stack>
            ) : !isValid ? (
              <Typography variant="body2" color="text.secondary">
                {placeholder ||
                  "Validate an OpenAPI definition to view source."}
              </Typography>
            ) : sourceError ? (
              <Typography variant="body2" color="error">
                {sourceError}
              </Typography>
            ) : !raw ? (
              <Typography variant="body2" color="text.secondary">
                No source to display.
              </Typography>
            ) : (
              <Box
                component="pre"
                sx={{
                  m: 0,
                  p: 2,
                  borderRadius: 1.5,
                  border: "1px solid",
                  borderColor: "divider",
                  bgcolor: "background.paper",
                  fontSize: 12,
                  lineHeight: 1.6,
                  whiteSpace: "pre",
                  overflow: "auto",
                  maxHeight: maxHeight - 30,
                }}
              >
                {raw}
              </Box>
            )}
          </Box>
        ) : (
            <SwaggerPreview
              title={title}
              definitionUrl={definitionUrl}
              definitionFile={definitionFile}
              isValid={isValid}
              isLoading={isLoading}
              placeholder={placeholder}
              maxHeight={maxHeight}
              apiEndpoint={apiEndpoint}
              minHeight={500}
            />
        )}
      </Box>
    </Paper>
  );
};

export default SwaggerPreviewWithSource;
