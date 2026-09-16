/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {
  buildSchema,
  GraphQLEnumType,
  GraphQLInputObjectType,
  GraphQLInterfaceType,
  GraphQLObjectType,
  GraphQLScalarType,
  GraphQLUnionType,
  isSpecifiedScalarType,
  printSchema,
  type GraphQLField,
  type GraphQLInputField,
  type GraphQLNamedType,
  type GraphQLSchema,
} from 'graphql';

/**
 * Parses SDL into a real `graphql-js` schema, rather than the regex-based
 * approach `api-portal/libs/graphql-schema-viewer` uses — a proper parser
 * handles directives, descriptions and edge cases a regex would miss.
 */
export const parseGraphQLSdl = (
  sdl: string,
): { schema: GraphQLSchema } | { error: string } => {
  try {
    return { schema: buildSchema(sdl) };
  } catch (error) {
    return { error: error instanceof Error ? error.message : String(error) };
  }
};

/** Re-prints a schema in `graphql-js`'s canonical formatting ("Format" action). */
export const formatSdl = (schema: GraphQLSchema): string => printSchema(schema);

export type GraphQLFieldSummary = {
  name: string;
  /** `(arg: Type, other: Type)`, or `''` when the field takes no arguments. */
  args: string;
  /** The field's return type, e.g. `[Country!]!`. */
  type: string;
  deprecated: boolean;
};

const describeArgs = (field: GraphQLField<unknown, unknown>): string => {
  if (field.args.length === 0) return '';
  return `(${field.args.map((arg) => `${arg.name}: ${arg.type.toString()}`).join(', ')})`;
};

const describeFields = (
  fields: Record<string, GraphQLField<unknown, unknown>>,
): GraphQLFieldSummary[] =>
  Object.values(fields).map((field) => ({
    args: describeArgs(field),
    deprecated: field.deprecationReason !== undefined && field.deprecationReason !== null,
    name: field.name,
    type: field.type.toString(),
  }));

/** Input object fields carry no arguments — a field of a field makes no sense in GraphQL. */
const describeInputFields = (fields: Record<string, GraphQLInputField>): GraphQLFieldSummary[] =>
  Object.values(fields).map((field) => ({
    args: '',
    deprecated: field.deprecationReason !== undefined && field.deprecationReason !== null,
    name: field.name,
    type: field.type.toString(),
  }));

/** Kinds shown in the explorer's type badges — mirrors the introspection `__TypeKind` names. */
export type GraphQLTypeKind =
  | 'OBJECT'
  | 'INPUT_OBJECT'
  | 'INTERFACE'
  | 'UNION'
  | 'ENUM'
  | 'SCALAR';

export type GraphQLTypeSummary = {
  kind: GraphQLTypeKind;
  name: string;
  description?: string;
  /** Field rows, for object-like kinds; absent for enum/union/scalar. */
  fields?: GraphQLFieldSummary[];
  /** Values, for an enum. */
  enumValues?: string[];
  /** Member type names, for a union. */
  unionMembers?: string[];
};

const kindOf = (type: GraphQLNamedType): GraphQLTypeKind | undefined => {
  if (type instanceof GraphQLObjectType) return 'OBJECT';
  if (type instanceof GraphQLInputObjectType) return 'INPUT_OBJECT';
  if (type instanceof GraphQLInterfaceType) return 'INTERFACE';
  if (type instanceof GraphQLUnionType) return 'UNION';
  if (type instanceof GraphQLEnumType) return 'ENUM';
  if (type instanceof GraphQLScalarType) return 'SCALAR';
  return undefined;
};

const summarizeType = (type: GraphQLNamedType): GraphQLTypeSummary | undefined => {
  const kind = kindOf(type);
  if (kind === undefined) return undefined;

  const base = { description: type.description ?? undefined, kind, name: type.name };

  if (type instanceof GraphQLObjectType || type instanceof GraphQLInterfaceType) {
    return { ...base, fields: describeFields(type.getFields()) };
  }
  if (type instanceof GraphQLInputObjectType) {
    return { ...base, fields: describeInputFields(type.getFields()) };
  }
  if (type instanceof GraphQLEnumType) {
    return { ...base, enumValues: type.getValues().map((value) => value.name) };
  }
  if (type instanceof GraphQLUnionType) {
    return { ...base, unionMembers: type.getTypes().map((member) => member.name) };
  }
  return base;
};

export type GraphQLSchemaSummary = {
  queryFields: GraphQLFieldSummary[];
  mutationFields: GraphQLFieldSummary[];
  subscriptionFields: GraphQLFieldSummary[];
  /** Every named type besides the root operation types and built-in scalars. */
  types: GraphQLTypeSummary[];
  deprecatedFieldCount: number;
};

/**
 * Everything the schema explorer and the "Schema loaded" summary banner need,
 * derived once from a parsed schema.
 */
export const summarizeSchema = (schema: GraphQLSchema): GraphQLSchemaSummary => {
  const queryType = schema.getQueryType();
  const mutationType = schema.getMutationType();
  const subscriptionType = schema.getSubscriptionType();
  const rootTypeNames = new Set(
    [queryType, mutationType, subscriptionType]
      .filter((type): type is NonNullable<typeof type> => type !== null && type !== undefined)
      .map((type) => type.name),
  );

  const types = Object.values(schema.getTypeMap())
    .filter((type) => !type.name.startsWith('__')) // introspection meta types
    .filter((type) => !rootTypeNames.has(type.name))
    .filter((type) => !isSpecifiedScalarType(type)) // String/Int/Float/Boolean/ID
    .map(summarizeType)
    .filter((summary): summary is GraphQLTypeSummary => summary !== undefined)
    .sort((a, b) => a.name.localeCompare(b.name));

  const queryFields = queryType ? describeFields(queryType.getFields()) : [];
  const mutationFields = mutationType ? describeFields(mutationType.getFields()) : [];
  const subscriptionFields = subscriptionType ? describeFields(subscriptionType.getFields()) : [];

  const deprecatedFieldCount = [...queryFields, ...mutationFields, ...subscriptionFields]
    .concat(types.flatMap((type) => type.fields ?? []))
    .filter((field) => field.deprecated).length;

  return { deprecatedFieldCount, mutationFields, queryFields, subscriptionFields, types };
};

/**
 * Total named types in a schema, introspection meta types (`__Type` and
 * friends) excluded — the headline count shown right after a schema resolves,
 * before the full explorer breakdown is needed.
 */
export const countNamedTypes = (schema: GraphQLSchema): number =>
  Object.keys(schema.getTypeMap()).filter((name) => !name.startsWith('__')).length;
