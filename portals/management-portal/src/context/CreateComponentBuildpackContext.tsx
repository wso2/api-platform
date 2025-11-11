import * as React from "react";

/* ---------- Types ---------- */
export type ProxyMetadata = {
  name: string;
  target?: string;
  context: string;
  version: string;
  description?: string;
  /** flips true once the user edits Context manually */
  contextEdited?: boolean;
};

type Setter<T> =
  | React.Dispatch<React.SetStateAction<T>>
  | ((next: T | ((prev: T) => T)) => void);

type Ctx = {
  /** API Contract (OAS upload) flow slice */
  contractMeta: ProxyMetadata;
  setContractMeta: Setter<ProxyMetadata>;
  resetContractMeta: () => void;

  /** Endpoint flow slice */
  endpointMeta: ProxyMetadata;
  setEndpointMeta: Setter<ProxyMetadata>;
  resetEndpointMeta: () => void;
};

const defaultMeta: ProxyMetadata = {
  name: "",
  target: "",
  context: "",
  version: "1.0.0",
  description: "",
  contextEdited: false,
};

/* ---------- Context ---------- */
const CreateComponentBuildpackContext = React.createContext<Ctx | null>(null);

export const CreateComponentBuildpackProvider: React.FC<
  React.PropsWithChildren<{
    initialContract?: Partial<ProxyMetadata>;
    initialEndpoint?: Partial<ProxyMetadata>;
  }>
> = ({ children, initialContract, initialEndpoint }) => {
  const [contractMeta, setContractMeta] = React.useState<ProxyMetadata>({
    ...defaultMeta,
    ...(initialContract ?? {}),
  });

  const [endpointMeta, setEndpointMeta] = React.useState<ProxyMetadata>({
    ...defaultMeta,
    ...(initialEndpoint ?? {}),
  });

  const resetContractMeta = React.useCallback(
    () => setContractMeta({ ...defaultMeta }),
    []
  );
  const resetEndpointMeta = React.useCallback(
    () => setEndpointMeta({ ...defaultMeta }),
    []
  );

  const value = React.useMemo<Ctx>(
    () => ({
      contractMeta,
      setContractMeta,
      resetContractMeta,
      endpointMeta,
      setEndpointMeta,
      resetEndpointMeta,
    }),
    [contractMeta, endpointMeta]
  );

  return (
    <CreateComponentBuildpackContext.Provider value={value}>
      {children}
    </CreateComponentBuildpackContext.Provider>
  );
};

export const useCreateComponentBuildpackContext = () => {
  const ctx = React.useContext(CreateComponentBuildpackContext);
  if (!ctx) {
    throw new Error(
      "useCreateComponentBuildpackContext must be used within CreateComponentBuildpackProvider"
    );
  }
  return ctx;
};
