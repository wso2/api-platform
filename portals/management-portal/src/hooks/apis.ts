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
  name: string;
  description?: string;
  request: {
    method: string;
    path: string;
    authentication?: Record<string, unknown>;
    /** service references used by the request */
    ["backend-services"]?: Array<{ name: string }>;
  };
};

export type ApiSummary = {
  id: string;
  name: string;
  displayName?: string;
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
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
};

export type CreateApiPayload = {
  name: string;
  context: string;
  version: string;
  projectId: string;
  description?: string;
  backendServices?: ApiBackendService[];
  operations?: ApiOperation[];
  /** raw OAS (yaml/json) if your backend uses it */
  contract?: string;
};

export type ImportOpenApiRequest = {
  api: {
    name: string;
    context: string;
    version: string;
    projectId: string;
    target?: string;
    description?: string;
    backendServices?: ApiBackendService[];
  };
  url?: string;
  definition?: File;
};

/** ---------- Gateways bound to an API ---------- */
export type ApiGatewaySummary = {
  id: string;
  organizationId: string;
  name: string;
  displayName?: string;
  description?: string;
  vhost?: string;
  isCritical?: boolean;
  functionalityType?: string; // e.g., "regular"
  isActive?: boolean;
  createdAt?: string;
  updatedAt?: string;
};

type ApiGatewayListResponse = {
  count?: number;
  list?: ApiGatewaySummary[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
};

const mapBackendServices = (services?: ApiBackendService[]) => {
  if (!services) return undefined;

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
      const { backendServices, operations, contract, ...rest } = payload;

      const body: Record<string, unknown> = { ...rest };

      if (payload.description) {
        body.description = payload.description;
      }

      const mappedServices = mapBackendServices(backendServices);
      if (mappedServices && mappedServices.length > 0) {
        body["backend-services"] = mappedServices;
      }

      if (operations && operations.length > 0) {
        body.operations = operations;
      }

      if (contract) {
        body.contract = contract;
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

  /**
   * Fetch from /api/v1/apis with optional projectId filter:
   * - /api/v1/apis                 => all APIs in org
   * - /api/v1/apis?projectId=....  => APIs for a specific project
   */
  const fetchProjectApis = useCallback(
    async (projectId: string): Promise<ApiSummary[]> => {
      const { token, baseUrl } = getApiConfig();

      const url =
        projectId && projectId.length > 0
          ? `${baseUrl}/api/v1/apis?projectId=${encodeURIComponent(projectId)}`
          : `${baseUrl}/api/v1/apis`;

      const response = await fetch(url, {
        method: "GET",
        headers: {
          Authorization: `Bearer ${token}`,
        },
      });

      if (response.status === 404) {
        return [];
      }

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch APIs${projectId ? ` for project ${projectId}` : ""}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data = await response.json();

      // API may return either a raw array or a { list, count, pagination } wrapper
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

  /** Fetch gateways attached to a specific API id */
  const fetchApiGateways = useCallback(
    async (apiId: string): Promise<ApiGatewaySummary[]> => {
      const { token, baseUrl } = getApiConfig();

      const response = await fetch(
        `${baseUrl}/api/v1/apis/${encodeURIComponent(apiId)}/gateways`,
        {
          method: "GET",
          headers: { Authorization: `Bearer ${token}` },
        }
      );

      if (response.status === 404) {
        return [];
      }

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to fetch gateways for ${apiId}: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data =
        (await response.json()) as ApiGatewayListResponse | ApiGatewaySummary[];

      if (Array.isArray(data)) return data;
      if (data?.list && Array.isArray(data.list)) return data.list;

      return [];
    },
    []
  );

  const importOpenApi = useCallback(
    async (
      payload: ImportOpenApiRequest,
      opts?: { signal?: AbortSignal }
    ): Promise<void> => {
      const { token, baseUrl } = getApiConfig();

      const formData = new FormData();
      formData.append("api", JSON.stringify(payload.api));
      
      if (payload.url) {
        formData.append("url", payload.url);
      }
      if (payload.definition) {
        formData.append("definition", payload.definition);
      }

      const res = await fetch(`${baseUrl}/api/v1/import/open-api`, {
        method: "POST",
        headers: {
          Authorization: `Bearer ${token}`,
        },
        body: formData,
        signal: opts?.signal,
      });

      if (!res.ok) {
        let errorMessage = `Failed to import OpenAPI: ${res.status} ${res.statusText}`;
        try {
          const errorBody = await res.json();
          if (errorBody.message) {
            errorMessage = errorBody.message;
            if (errorBody.description) {
              errorMessage += `: ${errorBody.description}`;
            }
          }
        } catch (parseError) {
          console.warn('Failed to parse error response as JSON:', parseError);
        }
        throw new Error(errorMessage);
      }
    },
    []
  );

  return {
    createApi,
    fetchProjectApis,
    fetchApi,
    deleteApi,
    fetchApiGateways,
    importOpenApi,
  };
};
