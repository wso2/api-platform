import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

export type ApiEndpoint = {
  url: string;
  description?: string;
};

export type ApiBackendService = {
  name: string;
  isDefault?: boolean;
  endpoints: ApiEndpoint[];
  retries?: number;
};

export type ApiOperation = {
  name?: string;
  description?: string;
  request?: {
    method?: string;
    path?: string;
    authentication?: Record<string, unknown>;
  };
};

export type ApiSummary = {
  id: string;
  name: string;
  context: string;
  version: string;
  description?: string;
  provider?: string;
  projectId: string;
  organizationId?: string;
  createdAt?: string;
  updatedAt?: string;
  lifeCycleStatus?: string;
  type?: string;
  transport?: string[];
  operations?: ApiOperation[];
  backendServices?: ApiBackendService[];
};

type ApiListResponse = {
  list?: ApiSummary[];
  count?: number;
};

export type CreateApiPayload = {
  name: string;
  context: string;
  version: string;
  projectId: string;
  description?: string;
  backendServices?: ApiBackendService[];
};

const mapBackendServices = (services?: ApiBackendService[]) => {
  if (!services) {
    return undefined;
  }

  return services.map((service) => ({
    name: service.name,
    isDefault: service.isDefault ?? false,
    retries: service.retries ?? 0,
    endpoints: service.endpoints.map((endpoint) => ({
      url: endpoint.url,
      description: endpoint.description,
    })),
  }));
};

const normalizeApiSummary = (api: any): ApiSummary => {
  if (!api) return api;
  const backend =
    api.backendServices ??
    api["backend-services"] ??
    api.backends ??
    undefined;

  const operations = api.operations ?? [];
  const transport = api.transport ?? [];

  const normalized: ApiSummary = {
    ...api,
    backendServices: backend,
    operations,
    transport,
  };

  return normalized;
};

export const useApisApi = () => {
  const createApi = useCallback(
    async (payload: CreateApiPayload): Promise<ApiSummary> => {
      const { token, baseUrl } = getApiConfig();
      const { backendServices, ...rest } = payload;

      const body: Record<string, unknown> = {
        ...rest,
      };

      if (payload.description) {
        body.description = payload.description;
      }

      const mappedServices = mapBackendServices(backendServices);
      if (mappedServices && mappedServices.length > 0) {
        body["backend-services"] = mappedServices;
      }

      const response = await fetch(`${baseUrl}/api/v1/apis`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to create API: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: ApiSummary = await response.json();
      return normalizeApiSummary(data);
    },
    []
  );

  const fetchProjectApis = useCallback(
    async (projectId: string): Promise<ApiSummary[]> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/projects/${projectId}/apis`,
        {
          method: "GET",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        }
      );

      if (response.status === 404) {
        return [];
      }

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch APIs for project ${projectId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data = await response.json();
      if (Array.isArray(data)) {
        return (data as ApiSummary[]).map(normalizeApiSummary);
      }

      const wrapped = data as ApiListResponse;
      if (wrapped.list && Array.isArray(wrapped.list)) {
        return wrapped.list.map(normalizeApiSummary);
      }

      return [];
    },
    []
  );

  const fetchApi = useCallback(async (apiId: string): Promise<ApiSummary> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to fetch API ${apiId}: ${response.status} ${response.statusText} ${errorBody}`
      );
    }

    const data: ApiSummary = await response.json();
    return normalizeApiSummary(data);
  }, []);

  const deleteApi = useCallback(async (apiId: string): Promise<void> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/apis/${apiId}`, {
      method: "DELETE",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!response.ok && response.status !== 204) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to delete API ${apiId}: ${response.status} ${response.statusText} ${errorBody}`
      );
    }
  }, []);

  return {
    createApi,
    fetchProjectApis,
    fetchApi,
    deleteApi,
  };
};
