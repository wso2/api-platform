import type {
  ApiOperation,
  ApiPolicy,
  Api,
  ApiDetail,
  ApiStatus,
  CreateApiInput,
  HttpMethod,
} from '../../types/domain';

type AnyRecord = Record<string, unknown>;

const rec = (value: unknown): AnyRecord =>
  value && typeof value === 'object' ? (value as AnyRecord) : {};

const arr = (value: unknown): unknown[] => (Array.isArray(value) ? value : []);

const str = (value: unknown): string =>
  typeof value === 'string' ? value : value == null ? '' : String(value);

const optStr = (value: unknown): string | undefined => str(value) || undefined;

const HTTP_METHODS: HttpMethod[] = [
  'GET',
  'POST',
  'PUT',
  'DELETE',
  'PATCH',
  'HEAD',
  'OPTIONS',
];

const asMethod = (value: unknown): HttpMethod => {
  const m = str(value).toUpperCase() as HttpMethod;
  return HTTP_METHODS.includes(m) ? m : 'GET';
};

const toPolicy = (value: unknown): ApiPolicy => {
  const source = rec(value);
  return {
    name: str(source.name),
    version: str(source.version) || '1.0.0',
    executionCondition: optStr(source.executionCondition),
    params:
      source.params && typeof source.params === 'object'
        ? (source.params as Record<string, unknown>)
        : undefined,
  };
};

const toOperation = (value: unknown): ApiOperation => {
  const source = rec(value);
  const request = rec(source.request);
  return {
    name: optStr(source.name),
    description: optStr(source.description),
    method: asMethod(request.method),
    path: str(request.path) || '/',
    policies: arr(request.policies).map(toPolicy),
  };
};

// REST API lifeCycleStatus → console ApiStatus.
const lifeCycleToStatus = (value: unknown): ApiStatus => {
  switch (str(value).toUpperCase()) {
    case 'PUBLISHED':
      return 'ACTIVE';
    case 'STAGED':
      return 'PENDING';
    case 'CREATED':
      return 'DRAFT';
    case 'DEPRECATED':
    case 'RETIRED':
    case 'BLOCKED':
      return 'FAILED';
    default:
      return 'DRAFT';
  }
};

/**
 * platform-api REST API → console Api (kind API_PROXY). `id` is the handle;
 * `displayName` is the human name (older releases sent `name` — tolerated).
 */
export const restApiToApi = (value: unknown): Api => {
  const source = rec(value);
  const id = str(source.id);
  const displayName = str(source.displayName) || str(source.name) || id;
  return {
    id,
    projectId: str(source.projectId),
    name: displayName,
    displayName,
    handler: id,
    kind: 'API_PROXY',
    status: lifeCycleToStatus(source.lifeCycleStatus),
    description: optStr(source.description),
    version: optStr(source.version),
    createdAt: optStr(source.createdAt),
    updatedAt: optStr(source.updatedAt),
    httpBased: true,
  };
};

const upstreamUrl = (def: unknown): string | undefined => optStr(rec(def).url);

/** REST API → ApiDetail (Develop fields + raw object for merge-on-PUT). */
export const restApiToDetail = (value: unknown): ApiDetail => {
  const source = rec(value);
  const upstream = rec(source.upstream);
  return {
    ...restApiToApi(value),
    context: optStr(source.context),
    transport: arr(source.transport).map(str),
    operations: arr(source.operations).map(toOperation),
    policies: arr(source.policies).map(toPolicy),
    endpoints: {
      prodUrl: upstreamUrl(upstream.main),
      sandboxUrl: upstreamUrl(upstream.sandbox),
    },
    raw: source,
  };
};

/**
 * Builds a REST API PUT body by merging Develop edits back into the raw object
 * (preserving server-managed fields). `operations`/`policies`/`upstream` are
 * replaced from the edited detail.
 */
export const detailToRestApiBody = (
  detail: ApiDetail
): Record<string, unknown> => {
  const base = { ...(detail.raw || {}) };
  base.operations = detail.operations.map((op) => ({
    name: op.name,
    description: op.description,
    request: {
      method: op.method,
      path: op.path,
      ...(op.policies && op.policies.length > 0
        ? {
            policies: op.policies.map((p) => ({
              name: p.name,
              version: p.version,
              ...(p.executionCondition
                ? { executionCondition: p.executionCondition }
                : {}),
              ...(p.params ? { params: p.params } : {}),
            })),
          }
        : {}),
    },
  }));
  base.policies = detail.policies.map((p) => ({
    name: p.name,
    version: p.version,
    ...(p.executionCondition
      ? { executionCondition: p.executionCondition }
      : {}),
    ...(p.params ? { params: p.params } : {}),
  }));
  const main: Record<string, unknown> = { url: detail.endpoints.prodUrl };
  const upstream: Record<string, unknown> = { main };
  if (detail.endpoints.sandboxUrl) {
    upstream.sandbox = { url: detail.endpoints.sandboxUrl };
  }
  base.upstream = upstream;
  return base;
};

// --- create-body builders (console CreateApiInput → platform-api) ---

const buildUpstreamAuth = (
  auth: CreateApiInput['upstreamAuth']
): Record<string, unknown> | undefined => {
  if (!auth || !auth.type) return undefined;
  const out: Record<string, unknown> = { type: auth.type };
  if (auth.header) out.header = auth.header;
  if (auth.value) out.value = auth.value;
  return out;
};

const buildUpstream = (
  input: CreateApiInput
): Record<string, unknown> | undefined => {
  const prod = (input.prodUrl || '').trim();
  if (!prod) return undefined;
  const auth = buildUpstreamAuth(input.upstreamAuth);
  const main: Record<string, unknown> = { url: prod };
  if (auth) main.auth = auth;
  const upstream: Record<string, unknown> = { main };
  const sandbox = (input.sandboxUrl || '').trim();
  if (sandbox) {
    const sb: Record<string, unknown> = { url: sandbox };
    if (auth) sb.auth = auth;
    upstream.sandbox = sb;
  }
  return upstream;
};

/** REST API context: user value (trimmed) or the slug handle as a fallback. */
const restContext = (input: CreateApiInput): string =>
  (input.apiContext || '').trim() || input.name;

/**
 * The `CreateRESTAPIRequest` body. `displayName` is the human name; `id` is the
 * slug handle. `operations` is populated when creating from an imported OpenAPI
 * definition (platform-api has no server-side import, so operations are parsed
 * in the browser and sent here). Note platform-api requires `upstream`, so the
 * create form must supply at least a production URL (see buildUpstream).
 */
export const createRestApiBody = (
  input: CreateApiInput,
  projectId: string
): Record<string, unknown> => {
  const transport =
    input.transport && input.transport.length > 0
      ? input.transport
      : ['http', 'https'];
  const body: Record<string, unknown> = {
    id: input.name,
    displayName: input.displayName || input.name,
    context: restContext(input),
    version: input.version,
    projectId,
    transport,
  };
  if (input.description) body.description = input.description;
  const upstream = buildUpstream(input);
  if (upstream) body.upstream = upstream;
  if (input.operations && input.operations.length > 0) {
    body.operations = input.operations.map((op) => ({
      name: op.name,
      description: op.description,
      request: {
        method: op.method,
        path: op.path,
        ...(op.policies && op.policies.length > 0
          ? { policies: op.policies }
          : {}),
      },
    }));
  }
  return body;
};
