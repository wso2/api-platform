// src/pages/.../StepTwoApis.tsx
import React from "react";
import { Box, Typography, Tooltip } from "@mui/material";
import ContentCopyOutlinedIcon from "@mui/icons-material/ContentCopyOutlined";

import type { GatewayRecord } from "./types";
import { IconButton } from "../../components/src/components/IconButton";
import type { Components } from "react-markdown";
import ReactMarkdown from "react-markdown";

export default function StepTwoApis({
  gateways,
  onGoStep1,
  onGatewayActivated, // kept for compatibility, not used now
  notify,
  onGoStep3, // kept for compatibility, not used now
}: {
  gateways: GatewayRecord[];
  onGoStep1: () => void;
  onGatewayActivated: (id: string) => void;
  notify: (msg: string) => void;
  onGoStep3: () => void;
}) {
  // -------- Helpers for markdown code styling (same pattern as other pages) --------
  const BashSyntax = ({ text }: { text: string }) => {
    const pattern =
      /(https?:\/\/\S+)|(\$[A-Z0-9_]+)|\b(curl|bash)\b|(\s--?[a-zA-Z0-9-]+)|(\s\|\s)|\b(POST|GET|PUT|DELETE|PATCH)\b/g;

    const parts: React.ReactNode[] = [];
    let last = 0;
    let m: RegExpExecArray | null;

    while ((m = pattern.exec(text)) !== null) {
      if (m.index > last) parts.push(text.slice(last, m.index));
      const [full, url, env, cmd, flag, pipe, httpVerb] = m;

      let color = "#EDEDF0";
      if (url) color = "#79a8ff"; // URLs
      else if (env) color = "#7dd3fc"; // $ENV
      else if (cmd) color = cmd === "curl" ? "#b8e78b" : "#f5d67b"; // curl/bash
      else if (flag) color = "#a8acb3"; // flags
      else if (pipe) color = "#EDEDF0"; // |
      else if (httpVerb) color = "#f2a36b"; // HTTP verbs

      parts.push(
        <span key={m.index} style={{ color }}>
          {full}
        </span>
      );
      last = pattern.lastIndex;
    }
    if (last < text.length) parts.push(text.slice(last));
    return <>{parts}</>;
  };

  const mdComponentsForCmd: Components = {
    pre: ({ children }) => (
      <Box
        sx={{
          bgcolor: "#373842",
          color: "#EDEDF0",
          p: 2.5,
          borderRadius: 1,
          fontFamily:
            'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
          fontSize: 14,
          lineHeight: 1.6,
          overflowX: "auto",
          m: 0,
        }}
      >
        <pre style={{ margin: 0, whiteSpace: "pre" }}>{children}</pre>
      </Box>
    ),
    code: (props: any) => {
      const raw = String(props.children ?? "");
      const isBlock = raw.includes("\n");
      const { ref, node, ...rest } = props as any; // strip to avoid ref typing issues
      if (!isBlock) return <code {...rest}>{raw}</code>;
      return (
        <code {...rest}>
          <BashSyntax text={raw} />
        </code>
      );
    },
  };

  // -------- Display + Copy strings --------
  const CMD_DISPLAY_MD = `\`\`\`bash
curl -X POST http://localhost:9090/apis \\
  -H "Content-Type: application/yaml" \\
  --data-binary @- <<'EOF'
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: PetStore API
  version: v1.0
  context: /petstore
  upstreams:
    - url: https://petstore.swagger.io/v2
  operations:
    - method: GET
      path: /pet/{petId}
    - method: POST
      path: /pet
EOF
\`\`\``;

  const CMD_COPY = `curl -X POST http://localhost:9090/apis \\
  -H "Content-Type: application/yaml" \\
  --data-binary @- <<'EOF'
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: PetStore API
  version: v1.0
  context: /petstore
  upstreams:
    - url: https://petstore.swagger.io/v2
  operations:
    - method: GET
      path: /pet/{petId}
    - method: POST
      path: /pet
EOF`;

  // Copy command only (no details shown after copy)
  const copyCurl = async () => {
    try {
      await navigator.clipboard.writeText(CMD_COPY);
      notify("Command copied.");
    } catch {
      notify("Unable to copy command");
    }
  };

  if (gateways.length === 0) {
    return (
      <Box textAlign="center">
        <Typography variant="h6" mb={1} fontWeight={600} gutterBottom>
          Add Your APIs
        </Typography>
        <Typography color="text.secondary" sx={{ mb: 2 }}>
          You need at least one Gateway before adding APIs.
        </Typography>
        {/* You can keep your custom Button import if preferred */}
        <button
          onClick={onGoStep1}
          style={{
            padding: "6px 16px",
            borderRadius: 6,
            border: 0,
            background: "#059669",
            color: "#fff",
            cursor: "pointer",
            fontWeight: 600,
          }}
        >
          Go to Step 1
        </button>
      </Box>
    );
  }

  return (
    <Box py={1}>
      <Typography variant="h5" fontWeight={600}>
        Add Your APIs
      </Typography>

      <Box>
        <Typography variant="body2" sx={{ mb: 1 }}>
          Run this command locally
        </Typography>

        <Box sx={{ position: "relative" }}>
          <ReactMarkdown components={mdComponentsForCmd}>
            {CMD_DISPLAY_MD}
          </ReactMarkdown>

          <Tooltip title="Copy command">
            <IconButton
              onClick={copyCurl}
              sx={{
                position: "absolute",
                top: 10,
                right: 6,
              }}
            >
              <ContentCopyOutlinedIcon sx={{ color: "#fff", fill: "#fff" }} />
            </IconButton>
          </Tooltip>
        </Box>
      </Box>
    </Box>
  );
}
