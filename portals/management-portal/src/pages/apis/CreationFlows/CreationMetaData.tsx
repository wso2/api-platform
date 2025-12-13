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
  (val || "")
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .trim();

const majorFromVersion = (v: string) => {
  const m = (v || "").trim().match(/\d+/);
  return m?.[0] ?? "";
};

const buildIdentifierFromNameAndVersion = (name: string, version: string) => {
  const base = slugify(name);
  const major = majorFromVersion(version);
  return major ? `${base}-v${major}` : base;
};

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
type IdentifierOverride = { identifier?: string; force?: boolean };

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

  const [nameVersionValidating, setNameVersionValidating] =
    React.useState(false);
  const [identifierValidating, setIdentifierValidating] = React.useState(false);

  const lastCheckedNameVersionRef = React.useRef<{
    name: string;
    version: string;
  } | null>(null);

  const lastCheckedIdentifierRef = React.useRef<string | null>(null);
  const nameVersionTimerRef = React.useRef<number | null>(null);
  const identifierTimerRef = React.useRef<number | null>(null);

  const didInitValidateRef = React.useRef(false);
  const debounceMs = 2000;

  const onValidationChangeRef =
    React.useRef<Props["onValidationChange"]>(onValidationChange);
  React.useEffect(() => {
    onValidationChangeRef.current = onValidationChange;
  }, [onValidationChange]);

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

  const identifierDisplayValue = React.useMemo(() => {
    return (meta.identifier ?? "").trim();
  }, [meta.identifier]);

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

      try {
        setNameVersionValidating(true);

        const res = await validateNameVersion({
          name: effectiveName,
          version: effectiveVersion,
        });
        lastCheckedNameVersionRef.current = {
          name: effectiveName,
          version: effectiveVersion,
        };

        if (!res.valid) {
          setNameVersionError(
            `API with name ${effectiveName} and version ${effectiveVersion} already exists.`
          );
        } else {
          setNameVersionError(null);
        }
      } catch (e) {
        lastCheckedNameVersionRef.current = null;
        const msg =
          e instanceof Error
            ? e.message
            : "Failed to validate name and version.";
        setNameVersionError(msg);
      } finally {
        setNameVersionValidating(false);
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
    async (override?: IdentifierOverride) => {
      const effectiveIdentifier = slugify(
        (override?.identifier ?? identifierToValidate).trim()
      ).trim();

      if (!effectiveIdentifier) return;

      const last = lastCheckedIdentifierRef.current;
      if (!override?.force && last === effectiveIdentifier) return;

      try {
        setIdentifierValidating(true);

        const res = await validateIdentifier(effectiveIdentifier);
        lastCheckedIdentifierRef.current = effectiveIdentifier;

        if (!res.valid) {
          setIdentifierError(
            `API with identifier ${effectiveIdentifier} already exists.`
          );
        } else {
          setIdentifierError(null);
        }
      } catch (e) {
        lastCheckedIdentifierRef.current = null;
        const msg =
          e instanceof Error ? e.message : "Failed to validate identifier.";
        setIdentifierError(msg);
      } finally {
        setIdentifierValidating(false);
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
    (override?: IdentifierOverride) => {
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
    const trimmed = v.trim();
    const slug = slugify(v);

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
      nextPatch.identifier = buildIdentifierFromNameAndVersion(
        v,
        meta.version || ""
      );
    }

    setNameVersionError(null);
    change(nextPatch);

    scheduleNameVersionValidation({ name: v });
    scheduleIdentifierValidation();
  };

  React.useEffect(() => {
    onValidationChangeRef.current?.({
      nameVersionError,
      identifierError,
      hasError:
        !!nameVersionError ||
        !!identifierError ||
        nameVersionValidating ||
        identifierValidating,
    });
  }, [
    nameVersionError,
    identifierError,
    nameVersionValidating,
    identifierValidating,
  ]);

  React.useEffect(() => {
    if (meta.identifierEdited || isIdentifierEditing) return;

    const nameForId = (meta.displayName || meta.name || "").trim();
    const verForId = (meta.version || "").trim();

    if (!nameForId || !verForId) return;

    const expected = buildIdentifierFromNameAndVersion(nameForId, verForId);

    if ((meta.identifier || "").trim() !== expected) {
      change({ identifier: expected });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [
    meta.displayName,
    meta.name,
    meta.version,
    meta.identifierEdited,
    isIdentifierEditing,
  ]);

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
              onBlur={(() => flushIdentifierValidation({ force: true })) as any}
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
              aria-label="Edit identifier"
            />
          </Stack>
        </Grid>

        <Grid size={{ xs: 12, md: 3 }}>
          <VersionInput
            value={meta.version}
            onChange={(v: string) => {
              setNameVersionError(null);

              const shouldAuto = !meta.identifierEdited && !isIdentifierEditing;
              const nextIdentifier = shouldAuto
                ? buildIdentifierFromNameAndVersion(
                    meta.displayName || meta.name || "",
                    v
                  )
                : meta.identifier;

              change({ version: v, identifier: nextIdentifier });

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
