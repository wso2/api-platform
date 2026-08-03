import type { ReactNode } from 'react';

import type { ConsoleScope } from '../scope/ConsoleScopeProvider';

export type NavigationLevel = 'organization' | 'project' | 'api';

export type NavigationDefinition = {
  featureKey?: string;
  /** Sidebar section heading this item is grouped under. */
  group?: string;
  icon: ReactNode;
  id: string;
  isVisible?: (scope: ConsoleScope) => boolean;
  label: string;
  level: NavigationLevel;
  match?: (pathname: string) => boolean;
  order: number;
  to: (scope: ConsoleScope) => string | undefined;
};

export type NavigationItem = {
  group: string;
  icon: ReactNode;
  id: string;
  isActive: boolean;
  label: string;
  to: string;
};

/** A sidebar section: a heading plus the nav items under it (order preserved). */
export type NavigationGroup = {
  label: string;
  items: NavigationItem[];
};

export const NAVIGATION_GROUP_BY_LEVEL: Record<NavigationLevel, string> = {
  organization: 'Organization',
  project: 'Project',
  api: 'API',
};
