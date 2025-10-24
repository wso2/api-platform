import { useCallback } from "react";
import { getApiConfig } from "./apiConfig";

export type OrganizationPayload = {
  id: string;
  handle: string;
  name: string;
};

export type OrganizationResponse = {
  id: string;
  handle: string;
  name: string;
  createdAt: string;
  updatedAt: string;
};

type OrganizationListResponse = {
  count: number;
  list: OrganizationResponse[];
};

const DEFAULT_PAYLOAD: OrganizationPayload = {
  id: "15655e49-9833-4129-acd9-290538fa3cf6",
  handle: "acme",
  name: "acme",
};

/**
 * Hook that encapsulates the organization creation API call.
 * Consumers can optionally override the default payload values.
 */
export const useCreateOrganization = () => {
  const createOrganization = useCallback(
    async (
      overrides: Partial<OrganizationPayload> = {}
    ): Promise<OrganizationResponse> => {
      const { token, baseUrl } = getApiConfig();
      const payload: OrganizationPayload = {
        ...DEFAULT_PAYLOAD,
        ...overrides,
      };

      const response = await fetch(`${baseUrl}/api/v1/organizations`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(payload),
      });

      if (!response.ok) {
        const errorBody = await response.text();
        throw new Error(
          `Failed to create organization: ${response.status} ${response.statusText} ${errorBody}`
        );
      }

      const data: OrganizationResponse = await response.json();
      return data;
    },
    []
  );

  return { createOrganization };
};

/**
 * Hook for fetching organizations visible to the current user.
 */
type JwtClaims = {
  organization?: string;
  handle?: string;
  organizationHandle?: string;
  username?: string;
  email?: string;
  firstName?: string;
  lastName?: string;
  name?: string;
};

const decodeJwtClaims = (token: string): JwtClaims | null => {
  const parts = token.split(".");
  if (parts.length < 2) {
    return null;
  }

  try {
    const payload = parts[1]
      .replace(/-/g, "+")
      .replace(/_/g, "/")
      .concat("==".slice(0, (4 - (parts[1].length % 4)) % 4));
    const globalAtob =
      typeof globalThis !== "undefined"
        ? (globalThis as unknown as { atob?: typeof atob }).atob
        : undefined;

    const atobFn = typeof atob === "function" ? atob : globalAtob;

    if (!atobFn) {
      return null;
    }

    const json = atobFn(payload);
    return JSON.parse(json) as JwtClaims;
  } catch {
    return null;
  }
};

export const useOrganizationsApi = () => {
  const fetchOrganizations = useCallback(async (): Promise<OrganizationResponse[]> => {
    const { token, baseUrl } = getApiConfig();

    const response = await fetch(`${baseUrl}/api/v1/organizations`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (response.status === 404) {
      const claims = decodeJwtClaims(token);
      if (!claims || !claims.organization) {
        return [];
      }

      const fallbackHandle =
        claims.organizationHandle?.trim() ??
        claims.handle?.trim() ??
        claims.username?.trim() ??
        claims.email?.split("@")[0]?.trim() ??
        claims.organization.slice(0, 8);

      const fullName = [claims.firstName, claims.lastName]
        .filter((value): value is string => Boolean(value && value.trim()))
        .map((value) => value.trim())
        .join(" ")
        .trim();

      const fallbackName =
        claims.name?.trim() || fullName || fallbackHandle || "Organization";

      return [
        {
          id: claims.organization,
          handle: fallbackHandle || claims.organization,
          name: fallbackName,
          createdAt: "",
          updatedAt: "",
        },
      ];
    }

    if (!response.ok) {
      const errorBody = await response.text();
      throw new Error(
        `Failed to fetch organizations: ${response.status} ${response.statusText} ${errorBody}`
      );
    }

    const data = await response.json();
    if (Array.isArray(data)) {
      return data;
    }
    if (data && Array.isArray((data as OrganizationListResponse).list)) {
      return (data as OrganizationListResponse).list;
    }
    if (data && typeof data === "object") {
      return [data as OrganizationResponse];
    }
    return [];
  }, []);

  return { fetchOrganizations };
};
