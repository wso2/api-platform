import type { Api, ApiKind, ApiStatus } from '../../types/domain';

export const COMPONENT_KIND_LABEL: Record<ApiKind, string> = {
  API_PROXY: 'API Proxy',
  SERVICE: 'Service',
  WEB_APP: 'Web App',
};

export type ChipColor =
  | 'default'
  | 'primary'
  | 'success'
  | 'warning'
  | 'error'
  | 'info';

export const componentStatusColor = (status: ApiStatus): ChipColor => {
  switch (status) {
    case 'ACTIVE':
      return 'success';
    case 'PENDING':
      return 'warning';
    case 'FAILED':
      return 'error';
    default:
      return 'default';
  }
};

export type ApiGroups = {
  apiProxies: Api[];
  others: Api[];
};

/** Splits components into the "API Proxies" section and everything else. */
export const groupApisByKind = (components: Api[]): ApiGroups => ({
  apiProxies: components.filter((component) => component.kind === 'API_PROXY'),
  others: components.filter((component) => component.kind !== 'API_PROXY'),
});

export const filterApis = (components: Api[], search: string): Api[] => {
  const term = search.trim().toLowerCase();
  if (!term) return components;
  return components.filter((component) =>
    [component.displayName, component.name, component.description].some(
      (field) => field?.toLowerCase().includes(term)
    )
  );
};
