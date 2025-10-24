import { RESERVED_PATH_SEGMENTS } from "./routes";

export const ROOT_LEVEL_SEGMENTS = RESERVED_PATH_SEGMENTS;

export const splitPathSegments = (pathname: string): string[] =>
  pathname.split("/").filter(Boolean);

export const isRootLevelSegment = (segment?: string | null): boolean => {
  if (!segment) return false;
  return ROOT_LEVEL_SEGMENTS.has(segment.toLowerCase());
};

export const normalizeSegmentsForProject = (
  segments: string[]
): string[] => {
  if (segments.length === 0) {
    return ["overview"];
  }
  const [first, ...rest] = segments;
  if (!first || first.toLowerCase() === "projectoverview") {
    return ["overview", ...rest];
  }
  return [first, ...rest];
};

export const normalizeSegmentsForOrganization = (
  segments: string[]
): string[] => {
  if (segments.length === 0) {
    return ["overview"];
  }
  const [first, ...rest] = segments;
  if (!first) {
    return ["overview", ...rest];
  }
  if (first.toLowerCase() === "projectoverview") {
    return ["overview", ...rest];
  }
  return [first, ...rest];
};
