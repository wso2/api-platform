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

/**
 * Normalize a version string extracted from an OpenAPI document.
 *
 * Behaviour:
 * - Accepts a string (e.g. "v1.2.3", "ver4.1.2", "1", "1.0.0").
 * - Extracts the first numeric sequence containing digits and dots.
 * - If no numeric sequence found, defaults to "1.0".
 * - Reduces the version to one decimal place (major.minor) by taking
 *   the first two numeric components. Missing minor defaults to 0.
 * - Prefixes the result with a leading "v" and returns it.
 *
 * Examples:
 * - "v11.11.1" -> "v11.11"
 * - "ver4.1.2.4" -> "v4.1"
 * - "3" -> "v3.0"
 * - "foo" -> "v1.0"
 */
export function formatVersionToMajorMinor(version?: unknown): string {
  try {
    const raw = String(version ?? "").trim();

    // Find first sequence of digits and dots
    const match = raw.match(/\d+(?:\.\d+)*/);
    if (!match) return "v1.0";

    const numeric = match[0];
    const parts = numeric.split(".");

    const majorNum = parseInt(parts[0], 10);
    const minorNum = parts[1] ? parseInt(parts[1], 10) : 0;

    const major = Number.isFinite(majorNum) ? String(majorNum) : "1";
    const minor = Number.isFinite(minorNum) ? String(minorNum) : "0";

    return `v${major}.${minor}`;
  } catch (e) {
    return "v1.0";
  }
}

/**
 * Validate that a version string is in the canonical `v<major>.<minor>` form
 * where major and minor are non-negative integers (no leading +/signs).
 * Examples: `v1.0`, `v0.5`, `v10.2` -> true
 */
export function isValidMajorMinorVersion(version?: unknown): boolean {
  if (typeof version !== "string") return false;
  return /^v\d+\.\d+$/.test(version.trim());
}
