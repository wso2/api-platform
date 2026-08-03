import { ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import {
  alpha,
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
  CircularProgress,
  Collapse,
  Divider,
  FormControl,
  FormControlLabel,
  FormGroup,
  FormLabel,
  InputAdornment,
  MenuItem,
  Select,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
  useTheme,
} from '@wso2/oxygen-ui';
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Globe,
  Info,
  Lightbulb,
  Link,
  Upload,
} from '@wso2/oxygen-ui-icons-react';
import yaml from 'js-yaml';
import { useNavigate, useParams } from 'react-router-dom';

import { useCreateApi } from '../../api/hooks/useMvpQueries';
import { useNotifications } from '../../components/Notifications';
import { routes } from '../../routes/paths';
import type {
  ApiOperation,
  CreateApiInput,
  CreateApiSource,
  HttpMethod,
  UpstreamAuth,
} from '../../types/domain';
import { isValidUrl, methodColor } from './develop/developEdit';

const OAS_METHODS = [
  'get',
  'post',
  'put',
  'delete',
  'patch',
  'head',
  'options',
];

type OasParameter = {
  name: string;
  in: string;
  required: boolean;
  description: string;
  type: string;
};
type OasResponse = { code: string; description: string };
type OasOperation = {
  method: string;
  path: string;
  summary: string;
  description: string;
  operationId: string;
  tags: string[];
  deprecated: boolean;
  parameters: OasParameter[];
  responses: OasResponse[];
};
type OasTagGroup = {
  tag: string;
  description: string;
  operations: OasOperation[];
};
type OasSummary = {
  title: string;
  version: string;
  description: string;
  servers: string[];
  operations: OasOperation[];
  groups: OasTagGroup[];
  source: string;
};

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
const asArray = (value: unknown): unknown[] =>
  Array.isArray(value) ? value : [];
const asString = (value: unknown): string =>
  typeof value === 'string' ? value : '';

/** Resolves the displayable type of a parameter or schema node. */
const schemaType = (node: Record<string, unknown>): string => {
  const schema = asRecord(node.schema);
  const type = asString(schema.type) || asString(node.type);
  if (type === 'array') {
    const items = asRecord(schema.items ?? node.items);
    const itemType = asString(items.type);
    return itemType ? `${itemType}[]` : 'array';
  }
  const format = asString(schema.format) || asString(node.format);
  return type ? (format ? `${type} <${format}>` : type) : '';
};

/** Parses an OpenAPI definition (YAML or JSON) into a preview summary. */
const parseOas = (text: string): OasSummary => {
  const doc = asRecord(yaml.load(text));
  if (Object.keys(doc).length === 0) {
    throw new Error('The definition is empty or could not be parsed.');
  }
  if (!doc.openapi && !doc.swagger) {
    throw new Error('Missing "openapi"/"swagger" version field.');
  }
  const info = asRecord(doc.info);
  const paths = asRecord(doc.paths);

  // Servers: OpenAPI 3 `servers[].url`, falling back to Swagger 2 host/basePath.
  const servers: string[] = asArray(doc.servers)
    .map((server) => asString(asRecord(server).url))
    .filter(Boolean);
  if (servers.length === 0 && doc.host) {
    const scheme = asArray(doc.schemes)[0];
    servers.push(
      `${asString(scheme) || 'https'}://${asString(doc.host)}${asString(doc.basePath)}`
    );
  }

  // Tag order + descriptions as declared at the document level.
  const tagOrder: string[] = [];
  const tagDescriptions: Record<string, string> = {};
  for (const tag of asArray(doc.tags)) {
    const name = asString(asRecord(tag).name);
    if (name) {
      tagOrder.push(name);
      tagDescriptions[name] = asString(asRecord(tag).description);
    }
  }

  const operations: OasOperation[] = [];
  for (const [path, node] of Object.entries(paths)) {
    const item = asRecord(node);
    const sharedParams = asArray(item.parameters);
    for (const method of OAS_METHODS) {
      const op = item[method];
      if (!op) continue;
      const opRec = asRecord(op);
      const parameters: OasParameter[] = [
        ...sharedParams,
        ...asArray(opRec.parameters),
      ].map((param) => {
        const rec = asRecord(param);
        return {
          name: asString(rec.name),
          in: asString(rec.in),
          required: rec.required === true,
          description: asString(rec.description),
          type: schemaType(rec),
        };
      });
      const responses: OasResponse[] = Object.entries(
        asRecord(opRec.responses)
      ).map(([code, value]) => ({
        code,
        description: asString(asRecord(value).description),
      }));
      const tags = asArray(opRec.tags).map(asString).filter(Boolean);
      operations.push({
        method: method.toUpperCase(),
        path,
        summary: asString(opRec.summary),
        description: asString(opRec.description),
        operationId: asString(opRec.operationId),
        tags: tags.length > 0 ? tags : ['default'],
        deprecated: opRec.deprecated === true,
        parameters,
        responses,
      });
    }
  }

  // Group by tag, preserving declared tag order, then first-seen order.
  const groupMap = new Map<string, OasOperation[]>();
  for (const op of operations) {
    const tag = op.tags[0];
    const bucket = groupMap.get(tag) ?? [];
    bucket.push(op);
    groupMap.set(tag, bucket);
  }
  const orderedTags = [
    ...tagOrder.filter((tag) => groupMap.has(tag)),
    ...[...groupMap.keys()].filter((tag) => !tagOrder.includes(tag)),
  ];
  const groups: OasTagGroup[] = orderedTags.map((tag) => ({
    tag,
    description: tagDescriptions[tag] || '',
    operations: groupMap.get(tag) ?? [],
  }));

  return {
    title: asString(info.title) || 'Untitled API',
    version: asString(info.version),
    description: asString(info.description),
    servers,
    operations,
    groups,
    source: yaml.dump(doc, { indent: 2, lineWidth: 120 }),
  };
};

/** An expandable swagger-style row for a single operation in the OAS preview. */
function OperationRow({ op }: { op: OasOperation }) {
  const [open, setOpen] = useState(false);
  const hasDetail =
    Boolean(op.description) ||
    op.parameters.length > 0 ||
    op.responses.length > 0;
  const color = methodColor(op.method);
  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderLeft: '3px solid',
        borderLeftColor: color === 'default' ? 'divider' : `${color}.main`,
        borderRadius: 1,
        overflow: 'hidden',
      }}
    >
      <Stack
        alignItems="center"
        direction="row"
        onClick={() => hasDetail && setOpen((value) => !value)}
        spacing={1.25}
        sx={{
          cursor: hasDetail ? 'pointer' : 'default',
          px: 1.25,
          py: 0.875,
          '&:hover': hasDetail ? { bgcolor: 'action.hover' } : undefined,
        }}
      >
        <Chip
          color={methodColor(op.method)}
          label={op.method}
          size="small"
          sx={{ flex: 'none', fontWeight: 700, minWidth: 62 }}
        />
        <Typography
          noWrap
          sx={{
            fontFamily: 'monospace',
            fontSize: 13,
            fontWeight: 600,
            textDecoration: op.deprecated ? 'line-through' : 'none',
          }}
        >
          {op.path}
        </Typography>
        {op.summary && (
          <Typography
            color="text.secondary"
            noWrap
            sx={{ flex: 1, fontSize: 12.5 }}
          >
            {op.summary}
          </Typography>
        )}
        {hasDetail && (
          <Box
            sx={{
              color: 'text.disabled',
              display: 'flex',
              flex: 'none',
              ml: 'auto',
            }}
          >
            {open ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
          </Box>
        )}
      </Stack>
      <Collapse in={open} unmountOnExit>
        <Box sx={{ borderColor: 'divider', borderTop: '1px solid', p: 1.5 }}>
          {op.description && (
            <Typography
              color="text.secondary"
              sx={{ mb: 1.5, whiteSpace: 'pre-wrap' }}
              variant="body2"
            >
              {op.description}
            </Typography>
          )}
          {op.parameters.length > 0 && (
            <Box sx={{ mb: op.responses.length > 0 ? 1.5 : 0 }}>
              <Typography sx={{ fontWeight: 700, mb: 0.5 }} variant="caption">
                PARAMETERS
              </Typography>
              <Stack spacing={0.5}>
                {op.parameters.map((param, index) => (
                  <Stack
                    alignItems="baseline"
                    direction="row"
                    key={`${param.in}-${param.name}-${index}`}
                    spacing={1}
                  >
                    <Typography
                      sx={{
                        fontFamily: 'monospace',
                        fontSize: 12.5,
                        fontWeight: 600,
                      }}
                    >
                      {param.name}
                    </Typography>
                    <Chip
                      label={param.in}
                      size="small"
                      sx={{ height: 18, fontSize: 10 }}
                      variant="outlined"
                    />
                    {param.type && (
                      <Typography
                        color="text.secondary"
                        sx={{ fontFamily: 'monospace', fontSize: 11.5 }}
                      >
                        {param.type}
                      </Typography>
                    )}
                    {param.required && (
                      <Typography
                        color="error.main"
                        sx={{ fontSize: 11, fontWeight: 600 }}
                      >
                        required
                      </Typography>
                    )}
                    {param.description && (
                      <Typography
                        color="text.secondary"
                        noWrap
                        sx={{ fontSize: 11.5 }}
                      >
                        — {param.description}
                      </Typography>
                    )}
                  </Stack>
                ))}
              </Stack>
            </Box>
          )}
          {op.responses.length > 0 && (
            <Box>
              <Typography sx={{ fontWeight: 700, mb: 0.5 }} variant="caption">
                RESPONSES
              </Typography>
              <Stack spacing={0.5}>
                {op.responses.map((response) => (
                  <Stack
                    alignItems="baseline"
                    direction="row"
                    key={response.code}
                    spacing={1}
                  >
                    <Chip
                      color={
                        response.code.startsWith('2') ? 'success' : 'default'
                      }
                      label={response.code}
                      size="small"
                      sx={{
                        fontFamily: 'monospace',
                        fontWeight: 700,
                        height: 20,
                        minWidth: 46,
                      }}
                    />
                    {response.description && (
                      <Typography color="text.secondary" sx={{ fontSize: 12 }}>
                        {response.description}
                      </Typography>
                    )}
                  </Stack>
                ))}
              </Stack>
            </Box>
          )}
        </Box>
      </Collapse>
    </Box>
  );
}

type ProxyType = 'http' | 'graphql' | 'websocket' | 'websub' | 'grpc';
type Method = 'import' | 'endpoint' | 'scratch' | 'genai';
type AuthType = 'none' | 'basic' | 'bearer' | 'api-key';

const slugify = (value: string): string =>
  value
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-+/g, '-');

const isValidVersion = (value: string): boolean =>
  /^\d+\.\d+(\.\d+)?$/.test(value.trim());

// --- design icons (from the Claude Design) ---
const HttpIcon = () => (
  <Box
    sx={{
      alignItems: 'center',
      bgcolor: '#13385a',
      borderRadius: 1,
      color: '#fff',
      display: 'flex',
      fontSize: 11,
      fontWeight: 800,
      height: 38,
      justifyContent: 'center',
      letterSpacing: '.5px',
      width: 52,
    }}
  >
    HTTP
  </Box>
);
const GraphqlIcon = () => (
  <svg width="42" height="42" viewBox="0 0 50 50" fill="none">
    <polygon
      points="25,4 43,14.5 43,35.5 25,46 7,35.5 7,14.5"
      stroke="#e535ab"
      strokeWidth="2"
    />
    <line x1="25" y1="5" x2="42" y2="35" stroke="#e535ab" strokeWidth="1.6" />
    <line
      x1="42"
      y1="15.5"
      x2="8"
      y2="35.5"
      stroke="#e535ab"
      strokeWidth="1.6"
    />
    <line x1="8" y1="14.5" x2="25" y2="45" stroke="#e535ab" strokeWidth="1.6" />
    {[
      [25, 4],
      [43, 14.5],
      [43, 35.5],
      [25, 46],
      [7, 35.5],
      [7, 14.5],
    ].map(([cx, cy], i) => (
      <circle cx={cx} cy={cy} r="3" fill="#e535ab" key={i} />
    ))}
  </svg>
);
const WebSocketIcon = () => (
  <svg
    width="40"
    height="40"
    viewBox="0 0 44 44"
    fill="none"
    stroke="#1c9b94"
    strokeWidth="2.4"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path d="M16 6v8M28 6v8" />
    <path d="M11 14h22v5a11 11 0 0 1-22 0z" />
    <path d="M22 30v8" />
  </svg>
);
const WebSubIcon = () => (
  <svg
    width="40"
    height="40"
    viewBox="0 0 44 44"
    stroke="#2f7fe0"
    strokeWidth="2"
  >
    <g strokeLinecap="round">
      <line x1="22" y1="22" x2="22" y2="8" />
      <line x1="22" y1="22" x2="34" y2="15" />
      <line x1="22" y1="22" x2="34" y2="29" />
      <line x1="22" y1="22" x2="22" y2="36" />
      <line x1="22" y1="22" x2="10" y2="29" />
      <line x1="22" y1="22" x2="10" y2="15" />
    </g>
    <circle cx="22" cy="22" r="3.5" fill="#2f7fe0" stroke="none" />
  </svg>
);
const GrpcIcon = () => (
  <Box
    sx={{
      alignItems: 'center',
      bgcolor: 'action.selected',
      borderRadius: 1,
      color: 'text.secondary',
      display: 'flex',
      fontSize: 13,
      fontWeight: 800,
      height: 38,
      justifyContent: 'center',
      width: 54,
    }}
  >
    gRPC
  </Box>
);

const PROXY_TYPES: {
  key: ProxyType;
  title: string;
  description: string;
  icon: ReactNode;
  enabled: boolean;
}[] = [
  {
    key: 'http',
    title: 'HTTP',
    description: 'Expose REST and HTTP-based APIs.',
    icon: <HttpIcon />,
    enabled: true,
  },
  {
    key: 'graphql',
    title: 'GraphQL',
    description: 'Expose a GraphQL schema as an API.',
    icon: <GraphqlIcon />,
    enabled: false,
  },
  {
    key: 'websocket',
    title: 'WebSocket',
    description: 'Stream data over WebSocket connections.',
    icon: <WebSocketIcon />,
    enabled: false,
  },
  {
    key: 'websub',
    title: 'WebSub',
    description: 'Publish and subscribe to event streams.',
    icon: <WebSubIcon />,
    enabled: false,
  },
  {
    key: 'grpc',
    title: 'gRPC',
    description: 'Expose high-performance gRPC services.',
    icon: <GrpcIcon />,
    enabled: false,
  },
];

// --- method icons ---
const ImportIcon = () => (
  <svg width="46" height="42" viewBox="0 0 52 48" fill="none">
    <rect x="6" y="6" width="26" height="10" rx="3" fill="#23a39b" />
    <rect x="6" y="22" width="26" height="10" rx="3" fill="#46b377" />
    <circle cx="12" cy="11" r="1.7" fill="#fff" />
    <circle cx="12" cy="27" r="1.7" fill="#fff" />
    <path
      d="M32 11h9a2 2 0 0 1 2 2v22a2 2 0 0 1-2 2h-9"
      stroke="#aab6c0"
      strokeWidth="1.7"
    />
  </svg>
);
const EndpointIcon = () => (
  <svg
    width="44"
    height="42"
    viewBox="0 0 46 46"
    fill="none"
    stroke="#23a39b"
    strokeWidth="2.2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <circle cx="22" cy="22" r="6.5" />
    <path d="M22 9v-4M22 41v-4M9 22h-4M41 22h-4M31.5 12.5l2.8-2.8M9.7 34.3l2.8-2.8M31.5 31.5l2.8 2.8M9.7 9.7l2.8 2.8" />
  </svg>
);
const ScratchIcon = () => (
  <svg
    width="44"
    height="42"
    viewBox="0 0 46 46"
    fill="none"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <path
      d="M28 8l8 8-18 18-9 2 2-9z"
      fill="none"
      stroke="#46b377"
      strokeWidth="2.2"
    />
    <path d="M26 10l8 8" stroke="#46b377" strokeWidth="2.2" />
  </svg>
);
const GenAiIcon = () => (
  <svg
    width="46"
    height="40"
    viewBox="0 0 50 44"
    fill="none"
    stroke="#23a39b"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
  >
    <rect x="11" y="13" width="28" height="22" rx="7" />
    <line x1="25" y1="7" x2="25" y2="13" />
    <circle cx="25" cy="5.5" r="2.4" fill="#23a39b" stroke="none" />
    <circle cx="19.5" cy="24" r="2.6" fill="#46b377" stroke="none" />
    <circle cx="30.5" cy="24" r="2.6" fill="#46b377" stroke="none" />
    <path d="M7 21v6M43 21v6" />
  </svg>
);

const METHODS: {
  key: Method;
  title: string;
  icon: ReactNode;
  enabled: boolean;
}[] = [
  {
    key: 'import',
    title: 'Import API Contract',
    icon: <ImportIcon />,
    enabled: true,
  },
  {
    key: 'endpoint',
    title: 'Start with Endpoint',
    icon: <EndpointIcon />,
    enabled: true,
  },
  {
    key: 'scratch',
    title: 'Start from Scratch',
    icon: <ScratchIcon />,
    enabled: true,
  },
  {
    key: 'genai',
    title: 'Create with GenAI',
    icon: <GenAiIcon />,
    enabled: false,
  },
];

/** A selectable card used for both proxy-type and method tiles. */
function SelectCard({
  selected,
  disabled,
  onClick,
  children,
  height,
}: {
  selected: boolean;
  disabled?: boolean;
  onClick?: () => void;
  children: ReactNode;
  height?: number;
}) {
  return (
    <Box
      onClick={disabled ? undefined : onClick}
      role="button"
      sx={{
        bgcolor: 'background.paper',
        border: '1.5px solid',
        borderColor: selected ? 'primary.main' : 'divider',
        borderRadius: 2.5,
        boxShadow: (theme) =>
          selected ? `0 0 0 1px ${theme.palette.primary.main} inset` : 'none',
        cursor: disabled ? 'not-allowed' : 'pointer',
        flex: 1,
        minWidth: 0,
        opacity: disabled ? 0.55 : 1,
        p: 2.5,
        transition: 'border-color .15s, box-shadow .15s',
        ...(height ? { minHeight: height } : {}),
        '&:hover': disabled ? {} : { borderColor: 'primary.light' },
      }}
    >
      {children}
    </Box>
  );
}

export function ApiCreatePage() {
  const { orgHandle = '', projectHandler = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const mutation = useCreateApi();
  const theme = useTheme();

  const [tab, setTab] = useState<'type' | 'sample'>('type');
  const [phase, setPhase] = useState<'select' | 'source' | 'details'>('select');
  const [proxy, setProxy] = useState<ProxyType>('http');
  const [method, setMethod] = useState<Method>('import');

  // Source (import URL/file vs scratch).
  const [sourceMode, setSourceMode] = useState<
    'scratch' | 'import-url' | 'import-file'
  >('scratch');
  const [importUrl, setImportUrl] = useState('');
  const [importFile, setImportFile] = useState<File | null>(null);

  // OAS preview.
  const [oasSummary, setOasSummary] = useState<OasSummary | null>(null);
  const [oasError, setOasError] = useState<string | null>(null);
  const [oasLoading, setOasLoading] = useState(false);
  const [previewMode, setPreviewMode] = useState<'overview' | 'source'>(
    'overview'
  );

  // Details.
  const [displayName, setDisplayName] = useState('');
  const [name, setName] = useState('');
  // Track manual edits so OAS-derived prefill never clobbers user input.
  const displayNameTouched = useRef(false);
  const nameTouched = useRef(false);
  const versionTouched = useRef(false);
  const [version, setVersion] = useState('1.0.0');
  const [basePath, setBasePath] = useState('');
  const basePathTouched = useRef(false);
  const [prodUrl, setProdUrl] = useState('');
  const [sandboxUrl, setSandboxUrl] = useState('');
  const [description, setDescription] = useState('');

  // Advanced.
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [transportHttp, setTransportHttp] = useState(true);
  const [transportHttps, setTransportHttps] = useState(true);
  const [authType, setAuthType] = useState<AuthType>('none');
  const [authHeader, setAuthHeader] = useState('X-API-Key');
  const [authValue, setAuthValue] = useState('');

  const isImport = method === 'import';
  const isScratch = method === 'scratch';
  // Backend URL is collected in the source step for "endpoint", in the details
  // step for "scratch", and is left to the OpenAPI servers for "import".
  const showBackendInDetails = isScratch;

  const resetPreview = () => {
    setOasSummary(null);
    setOasError(null);
    setOasLoading(false);
  };

  const startMethod = (m: Method) => {
    setMethod(m);
    setSourceMode(m === 'import' ? 'import-url' : 'scratch');
    resetPreview();
    // "Start from scratch" has no source step.
    setPhase(m === 'scratch' ? 'details' : 'source');
  };

  const onDisplayName = (value: string) => {
    displayNameTouched.current = true;
    setDisplayName(value);
    if (!nameTouched.current) setName(slugify(value));
  };

  // Applies a parsed OAS as preview + prefills untouched detail fields.
  const applySummary = useCallback((summary: OasSummary) => {
    setOasSummary(summary);
    setOasError(null);
    if (!displayNameTouched.current && summary.title) {
      setDisplayName(summary.title);
      if (!nameTouched.current) setName(slugify(summary.title));
    }
    if (!versionTouched.current && summary.version) setVersion(summary.version);
  }, []);

  const loadFromFile = async (file: File) => {
    setOasLoading(true);
    setOasError(null);
    setOasSummary(null);
    try {
      applySummary(parseOas(await file.text()));
    } catch (error) {
      setOasError(
        error instanceof Error ? error.message : 'Could not read the file.'
      );
    } finally {
      setOasLoading(false);
    }
  };

  // Best-effort preview for a URL definition (debounced; tolerant of CORS).
  useEffect(() => {
    if (phase !== 'source' || !isImport || sourceMode !== 'import-url') return;
    const url = importUrl.trim();
    if (!url || !isValidUrl(url)) {
      setOasSummary(null);
      setOasError(null);
      setOasLoading(false);
      return;
    }
    let cancelled = false;
    setOasLoading(true);
    setOasError(null);
    const timer = setTimeout(() => {
      void (async () => {
        try {
          const response = await fetch(url);
          if (!response.ok)
            throw new Error(`Request failed (${response.status})`);
          const text = await response.text();
          if (cancelled) return;
          applySummary(parseOas(text));
        } catch {
          if (cancelled) return;
          setOasSummary(null);
          setOasError(
            "Couldn't fetch this URL for preview (it may block cross-origin requests). It will still be imported when you create the API."
          );
        } finally {
          if (!cancelled) setOasLoading(false);
        }
      })();
    }, 600);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [importUrl, sourceMode, isImport, phase, applySummary]);

  const importReady =
    sourceMode === 'import-url'
      ? isValidUrl(importUrl) && importUrl.trim() !== ''
      : !!importFile;
  const endpointReady =
    isValidUrl(prodUrl) && prodUrl.trim() !== '' && isValidUrl(sandboxUrl);
  const sourceValid = isImport ? importReady : endpointReady;

  const authValid = authType === 'none' || authValue.trim() !== '';
  const detailsValid =
    displayName.trim() !== '' &&
    name.trim() !== '' &&
    isValidVersion(version) &&
    authValid &&
    (!showBackendInDetails ||
      (isValidUrl(prodUrl) && prodUrl.trim() !== '' && isValidUrl(sandboxUrl)));

  // Gateway routing path. Defaults to `/default/{identifier}/v{version}` and
  // stays in sync with those fields until the user edits it directly.
  const defaultBasePath = name ? `/default/${name}/v${version || '1.0.0'}` : '';
  useEffect(() => {
    if (!basePathTouched.current) setBasePath(defaultBasePath);
  }, [defaultBasePath]);

  const buildSource = (): CreateApiSource => {
    if (!isImport) return { mode: 'scratch' };
    if (sourceMode === 'import-url')
      return { mode: 'import-url', url: importUrl.trim() };
    if (importFile) return { mode: 'import-file', file: importFile };
    return { mode: 'scratch' };
  };

  const buildUpstreamAuth = (): UpstreamAuth | undefined => {
    if (authType === 'none') return undefined;
    return {
      type: authType,
      ...(authType === 'api-key'
        ? { header: authHeader.trim() || 'X-API-Key' }
        : {}),
      value: authValue.trim() || undefined,
    };
  };

  const submit = async () => {
    const transport = [
      ...(transportHttp ? ['http'] : []),
      ...(transportHttps ? ['https'] : []),
    ];
    // platform-api has no server-side OpenAPI import: on import, carry the
    // operations parsed from the definition so createApi sends them in the
    // structured create body. Fall back to the definition's first server URL
    // for the upstream when the user left the backend URL blank.
    const importedOperations: ApiOperation[] | undefined =
      isImport && oasSummary
        ? oasSummary.operations.map((op) => ({
            name: op.operationId || `${op.method} ${op.path}`,
            description: op.summary || op.description || undefined,
            method: op.method as HttpMethod,
            path: op.path,
          }))
        : undefined;
    const prod =
      prodUrl.trim() ||
      (isImport ? oasSummary?.servers[0]?.trim() || '' : '');
    const input: CreateApiInput = {
      name: name.trim(),
      displayName: displayName.trim(),
      description: description.trim() || undefined,
      kind: 'API_PROXY',
      version: version.trim(),
      apiContext: basePath.trim() || undefined,
      prodUrl: prod || undefined,
      sandboxUrl: sandboxUrl.trim() || undefined,
      upstreamAuth: buildUpstreamAuth(),
      transport: transport.length > 0 ? transport : undefined,
      source: buildSource(),
      operations: importedOperations,
    };
    try {
      const created = await mutation.mutateAsync(input);
      notify('API created', 'success');
      navigate(routes.api(orgHandle, projectHandler, created.handler));
    } catch (error) {
      notify(
        error instanceof Error ? error.message : 'Unable to create API',
        'error'
      );
    }
  };

  const tabSx = (on: boolean) => ({
    borderBottom: '2px solid',
    borderColor: on ? 'primary.main' : 'transparent',
    color: on ? 'text.primary' : 'text.secondary',
    cursor: 'pointer',
    fontSize: 15,
    fontWeight: on ? 700 : 600,
    pb: 1.25,
    pt: 1.25,
  });

  return (
    <Box sx={{ maxWidth: 1180, mx: 'auto', py: 1 }}>
      <Button
        onClick={() => navigate(routes.projectHome(orgHandle, projectHandler))}
        startIcon={<ArrowLeft size={18} />}
        sx={{ color: 'text.secondary', mb: 1, px: 0 }}
        variant="text"
      >
        Back to Project Home
      </Button>

      {/* Tabs */}
      <Stack
        direction="row"
        spacing={3.5}
        sx={{ borderBottom: '1px solid', borderColor: 'divider', mb: 1 }}
      >
        <Box onClick={() => setTab('type')} sx={tabSx(tab === 'type')}>
          Select a Type
        </Box>
        <Box onClick={() => setTab('sample')} sx={tabSx(tab === 'sample')}>
          Try a Sample
        </Box>
      </Stack>

      {tab === 'sample' ? (
        <Box
          sx={{
            alignItems: 'center',
            border: '1px dashed',
            borderColor: 'divider',
            borderRadius: 2,
            color: 'text.secondary',
            display: 'flex',
            flexDirection: 'column',
            gap: 1,
            height: 320,
            justifyContent: 'center',
            mt: 3,
          }}
        >
          <Typography variant="body2">Sample API gallery</Typography>
          <Typography variant="caption">
            Switch back to “Select a Type” to create a new API.
          </Typography>
        </Box>
      ) : phase === 'select' ? (
        <>
          <Typography
            sx={{ fontWeight: 700, mb: 2, mt: 3 }}
            variant="subtitle1"
          >
            Create New
          </Typography>
          <Stack direction={{ xs: 'column', sm: 'row' }} spacing={2}>
            {PROXY_TYPES.map((t) => (
              <SelectCard
                disabled={!t.enabled}
                key={t.key}
                onClick={() => setProxy(t.key)}
                selected={t.enabled && proxy === t.key}
              >
                <Stack alignItems="center" direction="row" spacing={1.5}>
                  <Box
                    sx={{
                      alignItems: 'center',
                      display: 'flex',
                      flexShrink: 0,
                    }}
                  >
                    {t.icon}
                  </Box>
                  <Typography noWrap sx={{ fontWeight: 700 }}>
                    {t.title}
                  </Typography>
                  {!t.enabled && (
                    <Chip color="default" label="Soon" size="small" />
                  )}
                </Stack>
              </SelectCard>
            ))}
          </Stack>

          <Card sx={{ mt: 3 }} variant="outlined">
            <CardContent>
              <Typography
                color="text.secondary"
                sx={{ fontWeight: 600, mb: 2 }}
                variant="body2"
              >
                How do you want to start?
              </Typography>
              <Box
                sx={{
                  display: 'grid',
                  gap: 2,
                  gridTemplateColumns: { xs: '1fr 1fr', md: 'repeat(4, 1fr)' },
                }}
              >
                {METHODS.map((m) => (
                  <SelectCard
                    disabled={!m.enabled}
                    key={m.key}
                    onClick={() => startMethod(m.key)}
                    selected={false}
                  >
                    <Stack alignItems="center" spacing={2} sx={{ py: 1.5 }}>
                      {m.icon}
                      <Typography
                        sx={{ fontWeight: 600, textAlign: 'center' }}
                        variant="body2"
                      >
                        {m.title}
                      </Typography>
                      {!m.enabled && (
                        <Chip
                          color="default"
                          label="Coming soon"
                          size="small"
                        />
                      )}
                    </Stack>
                  </SelectCard>
                ))}
              </Box>
            </CardContent>
          </Card>
        </>
      ) : phase === 'source' ? (
        // --- source phase: contract (import) or backend URL (endpoint) ---
        <Box
          sx={{
            alignItems: 'start',
            display: 'grid',
            gap: 2.75,
            gridTemplateColumns: { xs: '1fr', md: '1fr 300px' },
            mt: 3,
          }}
        >
          {/* left: contract / endpoint card */}
          <Card variant="outlined">
            <CardContent sx={{ p: 3 }}>
              <Stack spacing={2.5}>
                <Box>
                  <Typography sx={{ fontWeight: 500 }} variant="h6">
                    {isImport ? 'Import API contract' : 'Backend endpoint'}
                  </Typography>
                  <Typography color="text.secondary" variant="body2">
                    {isImport
                      ? 'Provide an OpenAPI definition to create the API from.'
                      : 'Point the gateway at the backend you want to expose.'}
                  </Typography>
                </Box>

                {isImport ? (
                  <Box>
                    <Typography
                      color="text.secondary"
                      sx={{ display: 'block', fontWeight: 500, mb: 1 }}
                      variant="caption"
                    >
                      API contract
                    </Typography>
                    <ToggleButtonGroup
                      exclusive
                      onChange={(
                        _event,
                        next: 'import-url' | 'import-file' | null
                      ) => {
                        if (next) {
                          setSourceMode(next);
                          resetPreview();
                        }
                      }}
                      size="small"
                      sx={{ mb: 2 }}
                      value={
                        sourceMode === 'import-file'
                          ? 'import-file'
                          : 'import-url'
                      }
                    >
                      <ToggleButton value="import-url">
                        <Link size={15} style={{ marginRight: 8 }} />
                        From URL
                      </ToggleButton>
                      <ToggleButton value="import-file">
                        <Upload size={15} style={{ marginRight: 8 }} />
                        Upload file
                      </ToggleButton>
                    </ToggleButtonGroup>

                    {sourceMode === 'import-file' ? (
                      <Stack alignItems="center" direction="row" spacing={1.5}>
                        <Button
                          component="label"
                          startIcon={<Upload size={16} />}
                          variant="outlined"
                        >
                          Choose file
                          <input
                            accept=".json,.yaml,.yml"
                            hidden
                            onChange={(event) => {
                              const file = event.target.files?.[0] ?? null;
                              setImportFile(file);
                              if (file) loadFromFile(file);
                              else resetPreview();
                            }}
                            type="file"
                          />
                        </Button>
                        <Typography color="text.secondary" variant="body2">
                          {importFile
                            ? importFile.name
                            : 'No file selected (.json, .yaml)'}
                        </Typography>
                      </Stack>
                    ) : (
                      <>
                        <TextField
                          error={importUrl !== '' && !isValidUrl(importUrl)}
                          fullWidth
                          onChange={(event) => setImportUrl(event.target.value)}
                          placeholder="https://petstore3.swagger.io/api/v3/openapi.json"
                          slotProps={{
                            input: {
                              startAdornment: (
                                <InputAdornment position="start">
                                  <Globe size={17} />
                                </InputAdornment>
                              ),
                              sx: { fontFamily: 'monospace', fontSize: 14 },
                            },
                          }}
                          value={importUrl}
                        />
                        <Stack
                          alignItems="center"
                          direction="row"
                          spacing={0.75}
                          sx={{ color: 'text.secondary', mt: 1 }}
                        >
                          <Info size={13} />
                          <Typography variant="caption">
                            URL to an OpenAPI 2.0 or 3.x definition (JSON or
                            YAML).
                          </Typography>
                        </Stack>
                      </>
                    )}
                  </Box>
                ) : (
                  <>
                    <TextField
                      error={prodUrl !== '' && !isValidUrl(prodUrl)}
                      fullWidth
                      helperText="The backend the gateway routes production traffic to."
                      label="Backend URL"
                      onChange={(event) => setProdUrl(event.target.value)}
                      placeholder="https://backend.example.com"
                      required
                      value={prodUrl}
                    />
                    <TextField
                      error={sandboxUrl !== '' && !isValidUrl(sandboxUrl)}
                      fullWidth
                      label="Sandbox backend URL (optional)"
                      onChange={(event) => setSandboxUrl(event.target.value)}
                      placeholder="https://sandbox.example.com"
                      value={sandboxUrl}
                    />
                  </>
                )}

                <Divider />
                <Stack
                  direction="row"
                  justifyContent="space-between"
                  spacing={2}
                >
                  <Button
                    onClick={() => setPhase('select')}
                    startIcon={<ArrowLeft size={16} />}
                    sx={{ borderRadius: 5 }}
                    variant="outlined"
                  >
                    Back
                  </Button>
                  <Button
                    disabled={!sourceValid}
                    endIcon={<ArrowRight size={16} />}
                    onClick={() => setPhase('details')}
                    sx={{ borderRadius: 5 }}
                    variant="contained"
                  >
                    Next
                  </Button>
                </Stack>
              </Stack>
            </CardContent>
          </Card>

          {/* loaded-definition preview — full width, below the import card */}
          {isImport && (oasLoading || oasSummary || oasError) && (
            <Box
              sx={{
                border: '1px solid',
                borderColor: 'divider',
                borderRadius: 2,
                gridColumn: { md: '1 / -1' },
                gridRow: { md: 2 },
                overflow: 'hidden',
              }}
            >
              <Box
                sx={{
                  alignItems: 'center',
                  borderBottom: '1px solid',
                  borderColor: 'divider',
                  display: 'flex',
                  gap: 1,
                  justifyContent: 'space-between',
                  px: 2,
                  py: 1.5,
                }}
              >
                <Box sx={{ minWidth: 0 }}>
                  <Typography noWrap sx={{ fontWeight: 600 }}>
                    {oasLoading
                      ? 'Loading definition…'
                      : oasSummary
                        ? oasSummary.title
                        : 'Preview unavailable'}
                  </Typography>
                  {oasSummary && (
                    <Stack
                      alignItems="center"
                      direction="row"
                      spacing={1}
                      sx={{ mt: 0.25 }}
                    >
                      {oasSummary.version && (
                        <Chip
                          label={`v${oasSummary.version}`}
                          size="small"
                          variant="outlined"
                        />
                      )}
                      <Typography color="text.secondary" variant="caption">
                        {oasSummary.operations.length} operations
                      </Typography>
                    </Stack>
                  )}
                </Box>
                {oasSummary && (
                  <ToggleButtonGroup
                    exclusive
                    onChange={(_event, next: 'overview' | 'source' | null) =>
                      next && setPreviewMode(next)
                    }
                    size="small"
                    value={previewMode}
                  >
                    <ToggleButton value="overview">Overview</ToggleButton>
                    <ToggleButton value="source">Source</ToggleButton>
                  </ToggleButtonGroup>
                )}
              </Box>
              <Box sx={{ maxHeight: 460, overflow: 'auto' }}>
                {oasLoading ? (
                  <Box
                    sx={{ display: 'flex', justifyContent: 'center', py: 5 }}
                  >
                    <CircularProgress size={22} />
                  </Box>
                ) : oasError ? (
                  <Box sx={{ p: 2 }}>
                    <Alert severity="warning">{oasError}</Alert>
                  </Box>
                ) : oasSummary && previewMode === 'overview' ? (
                  <Box sx={{ p: 1.5 }}>
                    {oasSummary.description && (
                      <Typography
                        color="text.secondary"
                        sx={{ mb: 1.5, whiteSpace: 'pre-wrap' }}
                        variant="body2"
                      >
                        {oasSummary.description}
                      </Typography>
                    )}
                    {oasSummary.servers.length > 0 && (
                      <Box sx={{ mb: 1.5 }}>
                        <Typography sx={{ fontWeight: 700 }} variant="caption">
                          SERVERS
                        </Typography>
                        <Stack spacing={0.25} sx={{ mt: 0.5 }}>
                          {oasSummary.servers.map((server) => (
                            <Typography
                              key={server}
                              sx={{ fontFamily: 'monospace', fontSize: 12 }}
                            >
                              {server}
                            </Typography>
                          ))}
                        </Stack>
                      </Box>
                    )}
                    {oasSummary.operations.length === 0 ? (
                      <Typography
                        color="text.secondary"
                        sx={{ p: 1 }}
                        variant="body2"
                      >
                        No operations defined in this definition.
                      </Typography>
                    ) : (
                      <Stack spacing={2}>
                        {oasSummary.groups.map((group) => (
                          <Box key={group.tag}>
                            {(oasSummary.groups.length > 1 ||
                              group.tag !== 'default') && (
                              <Stack
                                alignItems="baseline"
                                direction="row"
                                spacing={1}
                                sx={{ mb: 0.75 }}
                              >
                                <Typography
                                  sx={{
                                    fontWeight: 700,
                                    textTransform: 'capitalize',
                                  }}
                                  variant="subtitle2"
                                >
                                  {group.tag}
                                </Typography>
                                {group.description && (
                                  <Typography
                                    color="text.secondary"
                                    noWrap
                                    variant="caption"
                                  >
                                    {group.description}
                                  </Typography>
                                )}
                              </Stack>
                            )}
                            <Stack spacing={0.75}>
                              {group.operations.map((op, index) => (
                                <OperationRow
                                  key={`${op.method}-${op.path}-${index}`}
                                  op={op}
                                />
                              ))}
                            </Stack>
                          </Box>
                        ))}
                      </Stack>
                    )}
                  </Box>
                ) : oasSummary ? (
                  <Box
                    component="pre"
                    sx={{
                      bgcolor: 'action.hover',
                      fontFamily: 'monospace',
                      fontSize: 12,
                      lineHeight: 1.5,
                      m: 0,
                      p: 2,
                      whiteSpace: 'pre',
                    }}
                  >
                    {oasSummary.source}
                  </Box>
                ) : null}
              </Box>
            </Box>
          )}
          {/* right: contextual help */}
          <Stack spacing={2} sx={{ gridColumn: { md: 2 }, gridRow: { md: 1 } }}>
            <Box
              sx={{
                bgcolor: (t) => alpha(t.palette.info.main, 0.06),
                border: '1px solid',
                borderColor: (t) => alpha(t.palette.info.main, 0.25),
                borderRadius: 2,
                p: 2.25,
              }}
            >
              <Stack
                alignItems="center"
                direction="row"
                spacing={1}
                sx={{ mb: 1 }}
              >
                <Lightbulb color={theme.palette.info.main} size={17} />
                <Typography sx={{ fontWeight: 600 }}>
                  What happens next
                </Typography>
              </Stack>
              <Typography
                color="text.secondary"
                sx={{ lineHeight: 1.55 }}
                variant="body2"
              >
                {isImport
                  ? "We'll parse your definition, generate resources and schemas automatically, then let you review the endpoints before publishing."
                  : "We'll create an API proxy that routes traffic to your backend. You can add resources and policies after creation."}
              </Typography>
            </Box>

            {isImport && (
              <Box
                sx={{
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 2,
                  p: 2.25,
                }}
              >
                <Typography
                  color="text.secondary"
                  sx={{
                    display: 'block',
                    fontWeight: 600,
                    letterSpacing: '.12em',
                    mb: 1.5,
                  }}
                  variant="caption"
                >
                  SUPPORTED
                </Typography>
                <Stack spacing={1.25}>
                  {['OpenAPI 3.0 & 3.1', 'Swagger 2.0', 'JSON or YAML'].map(
                    (item) => (
                      <Stack
                        alignItems="center"
                        direction="row"
                        key={item}
                        spacing={1.25}
                      >
                        <CheckCircle2
                          color={theme.palette.success.main}
                          size={16}
                        />
                        <Typography variant="body2">{item}</Typography>
                      </Stack>
                    )
                  )}
                </Stack>
              </Box>
            )}
          </Stack>
        </Box>
      ) : (
        // --- details phase ---
        <Card sx={{ maxWidth: 900, mt: 3 }} variant="outlined">
          <CardContent sx={{ p: 3 }}>
            <Stack spacing={3.5}>
              <Box>
                <Typography sx={{ fontWeight: 700 }} variant="subtitle1">
                  Create an API Proxy
                </Typography>
                <Typography color="text.secondary" variant="body2">
                  Provide the details to configure and expose your API proxy.
                </Typography>
              </Box>

              {mutation.error && (
                <Alert severity="error">
                  {mutation.error instanceof Error
                    ? mutation.error.message
                    : 'Unable to create API'}
                </Alert>
              )}

              {/* Type badge */}
              <Stack
                alignItems="center"
                direction="row"
                spacing={1.75}
                sx={{
                  bgcolor: 'action.hover',
                  border: '1px solid',
                  borderColor: 'divider',
                  borderRadius: 1.5,
                  p: 1.75,
                }}
              >
                <HttpIcon />
                <Box>
                  <Typography sx={{ fontWeight: 600 }}>HTTP</Typography>
                  <Typography color="text.secondary" variant="body2">
                    Expose your service as an HTTP API proxy.
                  </Typography>
                </Box>
              </Stack>

              {/* Basic information */}
              <Box>
                <Typography
                  color="text.secondary"
                  sx={{
                    display: 'block',
                    fontWeight: 700,
                    letterSpacing: '.08em',
                    mb: 1.75,
                  }}
                  variant="caption"
                >
                  BASIC INFORMATION
                </Typography>
                <Box
                  sx={{
                    display: 'grid',
                    gap: 2,
                    gridTemplateColumns: { xs: '1fr', lg: 'repeat(3, 1fr)' },
                  }}
                >
                  <TextField
                    fullWidth
                    label="Display name"
                    onChange={(event) => onDisplayName(event.target.value)}
                    placeholder="Pizza Shack API"
                    required
                    value={displayName}
                  />
                  <TextField
                    fullWidth
                    helperText="URL-friendly. Auto-derived."
                    label="Identifier"
                    onChange={(event) => {
                      nameTouched.current = true;
                      setName(slugify(event.target.value));
                    }}
                    required
                    value={name}
                  />
                  <TextField
                    error={version !== '' && !isValidVersion(version)}
                    fullWidth
                    helperText="e.g. 1.0.0"
                    label="Version"
                    onChange={(event) => {
                      versionTouched.current = true;
                      setVersion(event.target.value);
                    }}
                    required
                    value={version}
                  />
                </Box>
                <TextField
                  fullWidth
                  helperText="Routing base path for this API. Defaults from the identifier and version."
                  label="Base Path"
                  onChange={(event) => {
                    basePathTouched.current = true;
                    setBasePath(event.target.value);
                  }}
                  placeholder="/default/identifier/v1.0.0"
                  sx={{ mt: 2 }}
                  value={basePath}
                />
                <TextField
                  fullWidth
                  label="Description"
                  minRows={2}
                  multiline
                  onChange={(event) => setDescription(event.target.value)}
                  placeholder="A short description of this API."
                  sx={{ mt: 2 }}
                  value={description}
                />
              </Box>

              {/* Backend endpoint */}
              {showBackendInDetails && (
                <Box>
                  <Typography
                    color="text.secondary"
                    sx={{
                      display: 'block',
                      fontWeight: 700,
                      letterSpacing: '.08em',
                      mb: 1.75,
                    }}
                    variant="caption"
                  >
                    BACKEND ENDPOINT
                  </Typography>
                  <Stack spacing={2}>
                    <TextField
                      error={prodUrl !== '' && !isValidUrl(prodUrl)}
                      fullWidth
                      helperText="The backend the gateway routes to."
                      label="Target URL"
                      onChange={(event) => setProdUrl(event.target.value)}
                      placeholder="https://backend.example.com"
                      required
                      value={prodUrl}
                    />
                    <TextField
                      error={sandboxUrl !== '' && !isValidUrl(sandboxUrl)}
                      fullWidth
                      label="Sandbox URL (optional)"
                      onChange={(event) => setSandboxUrl(event.target.value)}
                      placeholder="https://sandbox.example.com"
                      value={sandboxUrl}
                    />
                  </Stack>
                </Box>
              )}

              <Box>
                <Button
                  onClick={() => setShowAdvanced((s) => !s)}
                  size="small"
                  startIcon={
                    showAdvanced ? (
                      <ChevronDown size={16} />
                    ) : (
                      <ChevronRight size={16} />
                    )
                  }
                  variant="text"
                >
                  Advanced
                </Button>
                <Collapse in={showAdvanced}>
                  <Stack spacing={2.5} sx={{ pl: 1, pt: 1.5 }}>
                    <FormControl>
                      <FormLabel>Transport</FormLabel>
                      <FormGroup row>
                        <FormControlLabel
                          control={
                            <Checkbox
                              checked={transportHttp}
                              onChange={(event) =>
                                setTransportHttp(event.target.checked)
                              }
                            />
                          }
                          label="HTTP"
                        />
                        <FormControlLabel
                          control={
                            <Checkbox
                              checked={transportHttps}
                              onChange={(event) =>
                                setTransportHttps(event.target.checked)
                              }
                            />
                          }
                          label="HTTPS"
                        />
                      </FormGroup>
                    </FormControl>

                    <FormControl>
                      <FormLabel>Backend authentication</FormLabel>
                      <Typography
                        color="text.secondary"
                        sx={{ mb: 1 }}
                        variant="caption"
                      >
                        Credentials the gateway uses to call the backend.
                      </Typography>
                      <Select
                        onChange={(event) =>
                          setAuthType(event.target.value as AuthType)
                        }
                        size="small"
                        sx={{ maxWidth: 320 }}
                        value={authType}
                      >
                        <MenuItem value="none">None</MenuItem>
                        <MenuItem value="basic">Basic</MenuItem>
                        <MenuItem value="bearer">Bearer token</MenuItem>
                        <MenuItem value="api-key">API key</MenuItem>
                      </Select>
                      {authType === 'api-key' && (
                        <TextField
                          fullWidth
                          label="Header name"
                          onChange={(event) =>
                            setAuthHeader(event.target.value)
                          }
                          size="small"
                          sx={{ maxWidth: 320, mt: 1.5 }}
                          value={authHeader}
                        />
                      )}
                      {authType !== 'none' && (
                        <TextField
                          fullWidth
                          label={
                            authType === 'basic'
                              ? 'Base64 credentials'
                              : authType === 'bearer'
                                ? 'Token'
                                : 'API key value'
                          }
                          onChange={(event) => setAuthValue(event.target.value)}
                          required
                          size="small"
                          sx={{ maxWidth: 320, mt: 1.5 }}
                          type="password"
                          value={authValue}
                        />
                      )}
                    </FormControl>
                  </Stack>
                </Collapse>
              </Box>

              <Divider />
              <Stack direction="row" justifyContent="space-between" spacing={2}>
                <Button
                  disabled={mutation.isPending}
                  onClick={() => setPhase(isScratch ? 'select' : 'source')}
                >
                  Back
                </Button>
                <Button
                  disabled={!detailsValid || mutation.isPending}
                  onClick={submit}
                  variant="contained"
                >
                  {mutation.isPending ? 'Creating…' : 'Create'}
                </Button>
              </Stack>
            </Stack>
          </CardContent>
        </Card>
      )}
    </Box>
  );
}
