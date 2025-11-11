import * as React from "react";
import { Stack, Typography } from "@mui/material";

import {
  useCreateComponentBuildpackContext,
  type ProxyMetadata,
} from "../../../context/CreateComponentBuildpackContext";
import { TextInput } from "../../../components/src/components/TextInput";

const slugify = (val: string) =>
  val.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");

type Scope = "contract" | "endpoint";

type Props = {
  /**
   * Which slice of context to use.
   * Ignored if you pass value/onChange and use it as a controlled form.
   */
  scope: Scope;

  /** Optional controlled usage (overrides context). */
  value?: ProxyMetadata;
  onChange?: (next: ProxyMetadata) => void;

  readOnlyFields?: Partial<Record<keyof ProxyMetadata, boolean>>;
  title?: string;
};

const CreationMetaData: React.FC<Props> = ({
  scope,
  value,
  onChange,
  readOnlyFields,
  title,
}) => {
  const usingContext = !value && !onChange;
  const ctx = useCreateComponentBuildpackContext();

  const meta =
    value ??
    (scope === "contract" ? ctx.contractMeta : ctx.endpointMeta) ?? {
      name: "",
      target: "",
      context: "",
      version: "1.0.0",
      description: "",
      contextEdited: false,
    };

  const setMeta =
    onChange ??
    (scope === "contract" ? ctx.setContractMeta : ctx.setEndpointMeta);

  const change = (patch: Partial<ProxyMetadata>) =>
    setMeta({ ...meta, ...patch });

  const handleNameChange = (v: string) => {
    if (!meta.contextEdited) {
      const slug = slugify(v);
      change({ name: v, context: slug ? `/${slug}` : "" });
    } else {
      change({ name: v });
    }
  };

  const handleContextChange = (v: string) => {
    change({ context: v, contextEdited: true });
  };

  return (
    <Stack spacing={2}>
      {title ? (
        <Typography variant="subtitle2" sx={{ mb: 0.25 }}>
          {title}
        </Typography>
      ) : null}

      <TextInput
        label="Name"
        placeholder="Sample API"
        value={meta.name || ""}
        onChange={(v: string) => handleNameChange(v)}
        testId=""
        size="medium"
        disabled={!!readOnlyFields?.name}
      />

      <TextInput
        label="Target"
        placeholder="https://api.example.com/v1"
        value={meta.target ?? ""}
        onChange={(v: string) => change({ target: v })}
        testId=""
        size="medium"
        helperText="Base URL for your backend (used to create a default backend-service)."
        disabled={!!readOnlyFields?.target}
      />

      <TextInput
        label="Context"
        placeholder="/sample"
        value={meta.context || ""}
        onChange={(v: string) => handleContextChange(v)}
        testId=""
        size="medium"
        disabled={!!readOnlyFields?.context}
      />

      <TextInput
        label="Version"
        placeholder="1.0.0"
        value={meta.version || ""}
        onChange={(v: string) => change({ version: v })}
        testId=""
        size="medium"
        disabled={!!readOnlyFields?.version}
      />

      <TextInput
        label="Description"
        placeholder="Optional description"
        value={meta.description ?? ""}
        onChange={(v: string) => change({ description: v })}
        multiline
        testId=""
        disabled={!!readOnlyFields?.description}
      />
    </Stack>
  );
};

export default CreationMetaData;
