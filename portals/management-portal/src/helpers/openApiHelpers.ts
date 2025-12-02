/* ---------- Shared OpenAPI Import Helpers ---------- */

export function slugify(val: string) {
  return val.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
}

export function defaultServiceName(apiName: string) {
  const base = apiName?.trim() || "service";
  return `${slugify(base)}-service`;
}

export function firstServerUrl(api: any) {
  const services = api?.["backend-services"] || [];
  const endpoint = services[0]?.endpoints?.[0]?.url;
  return endpoint?.trim() || "";
}

export function deriveContext(api: any) {
  return api?.context || "/api";
}

export function mapOperations(
  operations: any[],
  options?: { serviceName?: string; withFallbackName?: boolean }
) {
  if (!Array.isArray(operations)) return [];
  
  return operations.map((op: any) => ({
    name: options?.withFallbackName 
      ? (op.name || (op.request?.method && op.request?.path
          ? `${op.request.method.toUpperCase()} ${op.request.path}`
          : op.request?.path || "Unknown"))
      : op.name,
    description: op.description,
    request: {
      method: op.request?.method || "GET",
      path: op.request?.path || "/",
      ...(options?.serviceName && { ["backend-services"]: [{ name: options.serviceName }] }),
    },
  }));
}
