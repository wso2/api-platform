import * as React from "react";
import { Stack, Typography, Grid } from "@mui/material";

import {
  useCreateComponentBuildpackContext,
  type ProxyMetadata,
} from "../../../context/CreateComponentBuildpackContext";
import { TextInput } from "../../../components/src/components/TextInput";
import VersionInput from "../../../common/VersionInput";
import { Button } from "../../../components/src/components/Button";
import Edit from "../../../components/src/Icons/generated/Edit";

const slugify = (val: string) =>
  val
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");

const buildIdentifierFromName = (name: string) => slugify(name);

type Scope = "contract" | "endpoint";

type Props = {
  scope: Scope;
  value?: ProxyMetadata;
  onChange?: (next: ProxyMetadata) => void;
  readOnlyFields?: Partial<Record<keyof ProxyMetadata | "identifier", boolean>>;
  title?: string;
};

const CreationMetaData: React.FC<Props> = ({
  scope,
  value,
  onChange,
  readOnlyFields,
  title,
}) => {
  const ctx = useCreateComponentBuildpackContext();

  const meta: ProxyMetadata & {
    identifier?: string;
    identifierEdited?: boolean;
  } = value ??
    (scope === "contract" ? ctx.contractMeta : ctx.endpointMeta) ?? {
      name: "",
      displayName: "",
      target: "",
      context: "",
      version: "1.0.0",
      description: "",
      contextEdited: false,
      identifier: "",
      identifierEdited: false,
    };

  const setMeta =
    onChange ??
    (scope === "contract" ? ctx.setContractMeta : ctx.setEndpointMeta);

  const change = (patch: Partial<ProxyMetadata>) =>
    setMeta({ ...meta, ...patch });
  const [isIdentifierEditing, setIsIdentifierEditing] = React.useState(
    !!meta.identifierEdited
  );

  React.useEffect(() => {
    if (
      meta.name &&
      !meta.identifier &&
      !meta.identifierEdited &&
      !isIdentifierEditing
    ) {
      change({
        identifier: buildIdentifierFromName(meta.name),
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleNameChange = (v: string) => {
    const slug = slugify(v);
    const trimmed = v.trim();

    const nextPatch: Partial<ProxyMetadata> & {
      identifier?: string;
      identifierEdited?: boolean;
    } = {
      name: slug || trimmed,
      displayName: v,
    };

    if (!meta.contextEdited) {
      nextPatch.context = slug ? `/${slug}` : "";
    }
    if (!meta.identifierEdited && !isIdentifierEditing) {
      nextPatch.identifier = buildIdentifierFromName(v);
    }

    change(nextPatch);
  };

  const handleContextChange = (v: string) => {
    change({ context: v, contextEdited: true });
  };

  const handleIdentifierChange = (v: string) => {
    change({
      identifier: slugify(v),
      identifierEdited: true,
    });
  };

  const handleIdentifierEditClick = () => {
    setIsIdentifierEditing(true);
    change({
      identifierEdited: true,
    });
  };
  const identifierDisabled =
    !!readOnlyFields?.["identifier"] || !isIdentifierEditing;

  return (
    <Stack spacing={2}>
      {title ? <Typography variant="subtitle2">{title}</Typography> : null}
      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 4 }}>
          <TextInput
            label="Name"
            placeholder="Sample API"
            value={meta.displayName || meta.name || ""}
            onChange={(v: string) => handleNameChange(v)}
            testId=""
            size="medium"
            disabled={!!readOnlyFields?.name}
          />
        </Grid>

        <Grid size={{ xs: 12, md: 5 }}>
          <Stack direction="row" spacing={1} alignItems="flex-end">
            <TextInput
              label="Identifier"
              placeholder="reading-list-api-rw"
              value={meta.identifier ?? ""}
              onChange={(v: string) => handleIdentifierChange(v)}
              testId=""
              size="medium"
              readonly={identifierDisabled}
            />
            <Button
              size="medium"
              startIcon={<Edit />}
              testId="identifier-edit"
              variant="link"
              onClick={handleIdentifierEditClick}
              disabled={!!readOnlyFields?.["identifier"]}
              style={{ marginBottom: 4 }}
            />
          </Stack>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          <VersionInput
            value={meta.version}
            onChange={(v: string) => change({ version: v })}
            disabled={!!readOnlyFields?.version}
            label="Version"
          />
        </Grid>
      </Grid>
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
