/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * Licensed under the Apache License, Version 2.0.
 */

export type ApiTypeFilter = 'async' | 'graphql' | 'grpc' | 'rest';

const API_KINDS: Record<ApiTypeFilter, string[]> = {
  async: ['async', 'asyncapi', 'event'],
  // `GraphQLAPI.kind` is the literal `GraphQLApi` (see openapi.yaml) — lowercased, `graphqlapi`.
  graphql: ['graphql', 'graphqlapi'],
  grpc: ['grpc'],
  rest: ['rest', 'restapi'],
};

export const matchesApiType = (kind: string | undefined, type: ApiTypeFilter): boolean =>
  API_KINDS[type].includes(kind?.toLowerCase() ?? '');
