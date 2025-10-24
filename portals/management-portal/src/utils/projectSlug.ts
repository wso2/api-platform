import { slugify } from "./slug";
import { RESERVED_PATH_SEGMENTS } from "./routes";

export const projectSlugFromName = (name: string, id: string): string => {
  const base = slugify(name);
  if (!base || RESERVED_PATH_SEGMENTS.has(base)) {
    return id.toLowerCase();
  }
  return base;
};

export const projectSlugMatches = (
  name: string,
  id: string,
  candidate: string
): boolean => {
  const normalized = candidate.toLowerCase();
  return (
    id.toLowerCase() === normalized ||
    projectSlugFromName(name, id) === normalized
  );
};
