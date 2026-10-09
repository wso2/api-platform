/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 * Licensed under the Apache License, Version 2.0.
 */

import type { GraphQLApiDetail } from '@/api/resources/graphqlApis';

/**
 * `schemaSource`/`sdl` to lay over a `GraphQLAPIDetail` for any metadata-only
 * PUT (`UpdateGraphQLAPI`) — resupplying it faithfully rather than forcing
 * `'introspection'`. The service's structural validation rejects a
 * `schemaSource` whose required field isn't also present, and forcing
 * `'introspection'` on an inline/url/file-sourced API would either hard-fail
 * that check (no reachable `upstream.main.url`) or silently re-derive the
 * schema from upstream, discarding what was actually authored.
 * `GraphQLAPIDetail` has no `sdl` field of its own (see `useGraphQLApiSdl`),
 * so for anything other than `'introspection'` this resupplies the
 * already-resolved SDL as `'inline'` — resolution re-validates that same text
 * and is a no-op, without ever touching upstream. `'introspection'` is the
 * one source `GraphQLAPIDetail` can always resupply as-is, since
 * `upstream.main.url` is already part of it.
 *
 * Shared by `GraphqlApiEditPage` (edits name/description/context/version/
 * endpoint together) and the Overview page's endpoint-only editor — both PUT
 * the same `GraphQLAPI` shape and must resupply this identically, or one of
 * them will eventually drift and start hard-failing this validation.
 */
export const resuppliedSchemaSource = (
  api: GraphQLApiDetail,
  sdl: string,
): { schemaSource: 'introspection' } | { schemaSource: 'inline'; sdl: string } =>
  api.schemaSource === 'introspection' || api.schemaSource === undefined
    ? { schemaSource: 'introspection' }
    : { schemaSource: 'inline', sdl };
