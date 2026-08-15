# Data access (`src/api`)

Every backend call is typed from `platform-api/resources/openapi.yaml`. Components import **hooks** and nothing else; ESLint enforces it.

>**A legacy layer still exists** (`client.ts`, `mvpApi.ts`, `useMvpQueries.ts`,
> `*/*Client.ts`) and still serves most pages. Don't add to it, port the page instead.

## Layers

```
component  →  hooks       resources/<name>/<name>.hooks.ts
                          scope binding, enabled gating, invalidation, optimistic updates
              queries     <name>.queries.ts
                          queryOptions: key + fetcher + staleTime, as plain values
              endpoints   <name>.endpoints.ts
                          one thin fn per spec operation, typed by operationId
              core/http   one axios instance — CSRF, X-Org-Id, timeouts, cancellation
                          ↓ BFF same-origin proxy → platform-api
```

A layer may only import from the one below it. Endpoints know nothing about the cache; queries know nothing about React; hooks know nothing about axios.

## Adding a resource

Copy `resources/restApis/`: three files, ~150 lines total.

```ts
// 1. endpoints — types come from the operationId, never hand-written
export type Thing = Schema<'Thing'>;
export const listThings = async (options?: RequestOptions) =>
  http.get<ResponseOf<'ListThings'>>('/things', { ...options, operationName: 'ListThings' });

// 2. queries — plain values, so loaders and prefetch can use them too
export const thingKeys = createResourceKeys('things');
export const thingQueries = {
  list: (org: OrgScope, query = {}) => queryOptions({
    queryKey: thingKeys.list(org, query),
    queryFn: ({ signal }) => listThings({ orgId: org, signal, query }),
    staleTime: staleTimes.standard,
  }),
};

// 3. hooks — the only thing components import
export const useThings = (filters = {}, overrides = {}) => {
  const { org } = useApiScope(overrides);
  return useQuery({ ...thingQueries.list(org!, filters), enabled: Boolean(org) });
};
```

Adding a spec operation? Run `npm run codegen` first. It regenerates `generated/platform.d.ts`, and CI fails if the committed output is stale.

## Rules

- **Derive types from `operationId`** — `ResponseOf` / `BodyOf` / `QueryOf` / `PathOf` from `core/spec`. If you're handwriting a request or response type, something is wrong.
- **Every scoped key is built from an `OrgScope`.** No `|| ''` fallbacks — that's how one tenant's cache ends up serving another.
- **Every `queryFn` forwards `signal`**, and every scoped query has an `enabled` gate. No throwing guards inside `queryFn`.
- **Branch on `ApiError.code`, not HTTP status.** Every failure is an `ApiError` with `code`, `fieldErrors`, `details` and `trackingId`.
- **Pick a `staleTime` tier deliberately** — `realtime` / `standard` / `stable` / `static`.
- **Mutations invalidate the resource root** and seed detail from their response,  *except* one-shot secrets (API keys, gateway tokens, secrets), which never enter the cache.
- **Lists read `pagination.total`**, never `list.length`.

## Testing

MSW at the network boundary — never stub a client or a hook. Toolkit in `src/test/msw` (`collection`, `resource`, `accepts`, `noContent`, `failure`, `recorder`, spec-typed fixtures); conventions in [`src/test/README.md`](../test/README.md).

```ts
server.use(collection('/things', [aThing()], { record: requests }));
```

- **Endpoint tests: one per resource** — URLs and params genuinely differ.
- **Hook tests: one per _shape_, not per resource**, the hooks are one template applied twelve times. Four files cover all of them; a new resource following an existing shape needs none.