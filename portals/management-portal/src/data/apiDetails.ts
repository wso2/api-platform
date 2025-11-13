import apis from "./apis.json";

export type ApiOperation = {
  method: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  path: string;
  summary: string;
  secured?: boolean;
};

export type ApiDetails = {
  id: string;
  protocol: string;
  summary: string;
  createdRelative: string;
  lifecycleStatus: string;
  compliancePercentage: number;
  deployment: {
    environment: "Development" | "Production" | "Sandbox";
    status: "Active" | "Inactive" | "Draft";
    lastUpdatedRelative: string;
    url: string;
  };
  operations: ApiOperation[];
};

const fallbackUrl =
  "https://apim.contoso.dev/apis/placeholder/{apiId}/v1/operations";

const defaultOperations: ApiOperation[] = [
  {
    method: "POST",
    path: "/books",
    summary: "Add a new book to the reading list",
  },
  {
    method: "GET",
    path: "/books/{id}",
    summary: "Get reading list book by id",
    secured: true,
  },
  {
    method: "PUT",
    path: "/books/{id}",
    summary: "Update a reading list book by id",
    secured: true,
  },
  {
    method: "DELETE",
    path: "/books/{id}",
    summary: "Delete a reading list book by id",
    secured: true,
  },
];

const baseById = new Map(apis.map((api) => [api.id, api]));

export const apiDetailsById: Record<string, ApiDetails> = {};

baseById.forEach((api, id) => {
  apiDetailsById[id] = {
    id,
    protocol: "HTTP",
    summary: api.description,
    createdRelative: "12 days ago",
    lifecycleStatus: "Created",
    compliancePercentage: 100,
    deployment: {
      environment: "Development",
      status: "Active",
      lastUpdatedRelative: "4 days ago",
      url: fallbackUrl.replace("{apiId}", api.context.replace("/", "")),
    },
    operations: defaultOperations,
  };
});

// Provide a richer example for the first API so the overview feels closer to the mock.
apiDetailsById["api-1"] = {
  ...apiDetailsById["api-1"],
  protocol: "HTTP",
  summary:
    "Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book.",
  deployment: {
    environment: "Development",
    status: "Active",
    lastUpdatedRelative: "4 days ago",
    url:
      "https://a9c4de22-c756-408a-925b-0db26e4a40b4-dev.e1-us-east-azure.preview-dv.bijiiraapis.dev/test1/reading-list-api",
  },
  operations: [
    {
      method: "POST",
      path: "/books",
      summary: "Add a new book to the reading list",
    },
    {
      method: "GET",
      path: "/books/{id}",
      summary: "Get reading list book by id",
      secured: true,
    },
    {
      method: "PUT",
      path: "/books/{id}",
      summary: "Update a reading list book by id",
      secured: true,
    },
    {
      method: "DELETE",
      path: "/books/{id}",
      summary: "Delete a reading list book by id",
      secured: true,
    },
  ],
};

export type ApiDetailsWithBase = ApiDetails & {
  name: string;
  owner: string;
  version: string;
  context: string;
  rating?: number;
};

export function buildApiDetails(id: string): ApiDetailsWithBase | undefined {
  const base = baseById.get(id);
  const details = apiDetailsById[id];
  if (!base || !details) {
    return undefined;
  }
  return {
    ...details,
    name: base.name,
    owner: base.owner,
    version: base.version,
    context: base.context,
    rating: base.rating,
  };
}
