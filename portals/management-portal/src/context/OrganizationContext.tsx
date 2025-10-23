import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  useCreateOrganization,
  useOrganizationsApi,
  type OrganizationPayload,
  type OrganizationResponse,
} from "../hooks/orgs";

type OrganizationContextValue = {
  organizations: OrganizationResponse[];
  selectedOrganization: OrganizationResponse | null;
  organization: OrganizationResponse | null;
  loading: boolean;
  error: string | null;
  refreshOrganizations: () => Promise<OrganizationResponse[]>;
  refreshOrganization: (
    overrides?: Partial<OrganizationPayload>
  ) => Promise<OrganizationResponse>;
  setSelectedOrganization: (organization: OrganizationResponse | null) => void;
};

const OrganizationContext = createContext<OrganizationContextValue | undefined>(
  undefined
);

type OrganizationProviderProps = {
  children: ReactNode;
  initialOverrides?: Partial<OrganizationPayload>;
};

export const OrganizationProvider = ({
  children,
  initialOverrides,
}: OrganizationProviderProps) => {
  const { createOrganization } = useCreateOrganization();
  const { fetchOrganizations } = useOrganizationsApi();
  const [organizations, setOrganizations] = useState<OrganizationResponse[]>([]);
  const [selectedOrganization, setSelectedOrganization] =
    useState<OrganizationResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const refreshOrganizations = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const result = await fetchOrganizations();
      setOrganizations(result);
      setSelectedOrganization((prev) => {
        if (prev) {
          const match = result.find((org) => org.id === prev.id);
          if (match) {
            return match;
          }
        }

        if (initialOverrides?.id) {
          const match = result.find((org) => org.id === initialOverrides.id);
          if (match) {
            return match;
          }
        }

        return result[0] ?? null;
      });
      return result;
    } catch (err) {
      if (err instanceof Error && err.message.includes("Organization not found")) {
        setOrganizations([]);
        setSelectedOrganization(null);
        setError(null);
        return [];
      }
      const message =
        err instanceof Error ? err.message : "Unknown error occurred";
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, [fetchOrganizations, initialOverrides?.id]);

  const refreshOrganization = useCallback(
    async (overrides?: Partial<OrganizationPayload>) => {
      setLoading(true);
      setError(null);

      try {
        const result = await createOrganization(overrides);
        setOrganizations((prev) => {
          const next = prev.filter((org) => org.id !== result.id);
          return [result, ...next];
        });
        setSelectedOrganization(result);
        return result;
      } catch (err) {
        const message =
          err instanceof Error ? err.message : "Unknown error occurred";
        setError(message);
        throw err;
      } finally {
        setLoading(false);
      }
    },
    [createOrganization]
  );

  useEffect(() => {
    // refreshOrganization(initialOverrides).catch(() => {
    //   /* errors are stored in state */
    // });
    refreshOrganizations().catch(() => {
      /* errors are stored in state */
    });
  }, [refreshOrganizations]);

  const value = useMemo<OrganizationContextValue>(
    () => ({
      organizations,
      organization: selectedOrganization,
      selectedOrganization,
      loading,
      error,
      refreshOrganizations,
      refreshOrganization,
      setSelectedOrganization,
    }),
    [
      organizations,
      selectedOrganization,
      loading,
      error,
      refreshOrganizations,
      refreshOrganization,
      setSelectedOrganization,
    ]
  );

  return (
    <OrganizationContext.Provider value={value}>
      {children}
    </OrganizationContext.Provider>
  );
};

export const useOrganization = () => {
  const context = useContext(OrganizationContext);

  if (!context) {
    throw new Error(
      "useOrganization must be used within an OrganizationProvider"
    );
  }

  return context;
};
