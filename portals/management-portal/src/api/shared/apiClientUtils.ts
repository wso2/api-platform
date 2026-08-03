export type GraphqlResponse<T> = T;
export type AnyRecord = Record<string, unknown>;

export const useMockApi = () => import.meta.env.VITE_USE_MOCK_API === 'true';

export const delay = () => new Promise((resolve) => setTimeout(resolve, 120));

export const asRecord = (value: unknown): AnyRecord =>
  value && typeof value === 'object' ? (value as AnyRecord) : {};

export const asArray = (value: unknown): unknown[] =>
  Array.isArray(value) ? value : [];

export const gqlString = (value: string) => JSON.stringify(value);
