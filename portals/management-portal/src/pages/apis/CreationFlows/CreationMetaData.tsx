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

import { useGithubProjectValidationContext } from "../../../context/validationContext";

const slugify = (val: string) =>
  val
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .trim();

const buildIdentifierFromName = (name: string) => slugify(name);

type Scope = "contract" | "endpoint";

type Props = {
  scope: Scope;
  value?: ProxyMetadata;
  onChange?: (next: ProxyMetadata) => void;
  readOnlyFields?: Partial<Record<keyof ProxyMetadata | "identifier", boolean>>;
  title?: string;
  onValidationChange?: (state: {
    nameVersionError: string | null;
    identifierError: string | null;
    hasError: boolean;
  }) => void;
};

type NameVersionOverride = { name?: string; version?: string; force?: boolean };

const CreationMetaData: React.FC<Props> = ({
  scope,
  value,
  onChange,
  readOnlyFields,
  title,
  onValidationChange,
}) => {
  const ctx = useCreateComponentBuildpackContext();
  const { validateNameVersion, validateIdentifier } =
    useGithubProjectValidationContext();

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
  const [nameVersionError, setNameVersionError] = React.useState<string | null>(
    null
  );
  const [identifierError, setIdentifierError] = React.useState<string | null>(
    null
  );

  const lastCheckedNameVersionRef = React.useRef<{
    name: string;
    version: string;
  } | null>(null);

  const lastCheckedIdentifierRef = React.useRef<string | null>(null);

  const nameVersionTimerRef = React.useRef<number | null>(null);
  const identifierTimerRef = React.useRef<number | null>(null);

  const didInitValidateRef = React.useRef(false);
  const debounceMs = 2000;

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
    change({ identifierEdited: true });
  };

  const versionMajor = React.useMemo(() => {
    const v = (meta.version || "").trim();
    const m = v.match(/\d+/);
    return m?.[0] ?? "";
  }, [meta.version]);

  const identifierDisplayValue = React.useMemo(() => {
    const base = (meta.identifier ?? "").trim();
    if (!base) return "";
    if (meta.identifierEdited) {
      return base;
    }
    if (!versionMajor) return base;
    if (/-v\d+$/i.test(base)) return base;

    return `${base}-v${versionMajor}`;
  }, [meta.identifier, meta.identifierEdited, versionMajor]);

  const identifierToValidate = React.useMemo(() => {
    return identifierDisplayValue.trim();
  }, [identifierDisplayValue]);

  const identifierDisabled =
    !!readOnlyFields?.["identifier"] || !isIdentifierEditing;

  const runNameVersionValidation = React.useCallback(
    async (override?: NameVersionOverride) => {
      const effectiveName = (
        override?.name ??
        (meta.displayName || meta.name || "")
      ).trim();

      const effectiveVersion = (
        override?.version ??
        (meta.version || "")
      ).trim();

      if (!effectiveName || !effectiveVersion) return;

      const last = lastCheckedNameVersionRef.current;
      if (
        !override?.force &&
        last &&
        last.name === effectiveName &&
        last.version === effectiveVersion
      ) {
        return;
      }

      lastCheckedNameVersionRef.current = {
        name: effectiveName,
        version: effectiveVersion,
      };

      try {
        const res = await validateNameVersion({
          name: effectiveName,
          version: effectiveVersion,
        });

        if (!res.valid) {
          setNameVersionError(
            `API with name ${effectiveName} and version ${effectiveVersion} already exists.`
          );
        } else {
          setNameVersionError(null);
        }
      } catch (e) {
        const msg =
          e instanceof Error
            ? e.message
            : "Failed to validate name and version.";
        setNameVersionError(msg);
      }
    },
    [meta.displayName, meta.name, meta.version, validateNameVersion]
  );

  const scheduleNameVersionValidation = React.useCallback(
    (override?: { name?: string; version?: string }) => {
      if (nameVersionTimerRef.current)
        window.clearTimeout(nameVersionTimerRef.current);

      nameVersionTimerRef.current = window.setTimeout(() => {
        void runNameVersionValidation(override);
      }, debounceMs);
    },
    [runNameVersionValidation, debounceMs]
  );

  const flushNameVersionValidation = React.useCallback(
    (override?: NameVersionOverride) => {
      if (nameVersionTimerRef.current) {
        window.clearTimeout(nameVersionTimerRef.current);
        nameVersionTimerRef.current = null;
      }
      void runNameVersionValidation(override);
    },
    [runNameVersionValidation]
  );

  const runIdentifierValidation = React.useCallback(
    async (override?: { identifier?: string }) => {
      const effectiveIdentifier = (
        override?.identifier ?? identifierToValidate
      ).trim();

      if (!effectiveIdentifier) return;

      const last = lastCheckedIdentifierRef.current;
      if (last === effectiveIdentifier) return;

      lastCheckedIdentifierRef.current = effectiveIdentifier;

      try {
        const res = await validateIdentifier(effectiveIdentifier);

        if (!res.valid) {
          setIdentifierError(
            `API with identifier ${effectiveIdentifier} already exists.`
          );
        } else {
          setIdentifierError(null);
        }
      } catch (e) {
        const msg =
          e instanceof Error ? e.message : "Failed to validate identifier.";
        setIdentifierError(msg);
      }
    },
    [identifierToValidate, validateIdentifier]
  );

  const scheduleIdentifierValidation = React.useCallback(
    (override?: { identifier?: string }) => {
      if (identifierTimerRef.current)
        window.clearTimeout(identifierTimerRef.current);

      identifierTimerRef.current = window.setTimeout(() => {
        void runIdentifierValidation(override);
      }, debounceMs);
    },
    [runIdentifierValidation, debounceMs]
  );

  const flushIdentifierValidation = React.useCallback(
    (override?: { identifier?: string }) => {
      if (identifierTimerRef.current) {
        window.clearTimeout(identifierTimerRef.current);
        identifierTimerRef.current = null;
      }
      void runIdentifierValidation(override);
    },
    [runIdentifierValidation]
  );

  React.useEffect(() => {
    return () => {
      if (nameVersionTimerRef.current)
        window.clearTimeout(nameVersionTimerRef.current);
      if (identifierTimerRef.current)
        window.clearTimeout(identifierTimerRef.current);
    };
  }, []);

  React.useEffect(() => {
    if (didInitValidateRef.current) return;
    didInitValidateRef.current = true;

    const effectiveName = (meta.displayName || meta.name || "").trim();
    const effectiveVersion = (meta.version || "").trim();

    if (effectiveName && effectiveVersion) {
      void runNameVersionValidation({
        name: effectiveName,
        version: effectiveVersion,
      });
    }

    if (identifierToValidate) {
      void runIdentifierValidation({ identifier: identifierToValidate });
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

    setNameVersionError(null);
    change(nextPatch);

    scheduleNameVersionValidation({ name: v });
    scheduleIdentifierValidation();
  };

  React.useEffect(() => {
    onValidationChange?.({
      nameVersionError,
      identifierError,
      hasError: !!nameVersionError || !!identifierError,
    });
  }, [nameVersionError, identifierError, onValidationChange]);

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
            onBlur={(() => flushNameVersionValidation({ force: true })) as any}
            testId=""
            size="medium"
            disabled={!!readOnlyFields?.name}
          />
        </Grid>

        <Grid size={{ xs: 12, md: 5 }}>
          <Stack direction="row" spacing={1} alignItems="flex-end">
            <TextInput
              label="Identifier"
              placeholder="reading-list-api-rw-v1"
              value={identifierDisplayValue}
              onChange={(v: string) => {
                handleIdentifierChange(v);
                setIdentifierError(null);
                scheduleIdentifierValidation({ identifier: v });
              }}
              onBlur={(() => flushIdentifierValidation()) as any}
              testId="Identifier"
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
              aria-label="Edit identifier"
            />
          </Stack>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          <VersionInput
            value={meta.version}
            onChange={(v: string) => {
              setNameVersionError(null);
              change({ version: v });

              scheduleNameVersionValidation({ version: v });
              setIdentifierError(null);
              scheduleIdentifierValidation();
            }}
            disabled={!!readOnlyFields?.version}
            label="Version"
          />
        </Grid>

        {nameVersionError || identifierError ? (
          <Grid size={{ xs: 12 }}>
            <Stack spacing={0.25} sx={{ mt: -1 }}>
              {nameVersionError ? (
                <Typography variant="caption" color="error">
                  {nameVersionError}
                </Typography>
              ) : null}
              {identifierError ? (
                <Typography variant="caption" color="error">
                  {identifierError}
                </Typography>
              ) : null}
            </Stack>
          </Grid>
        ) : null}
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
