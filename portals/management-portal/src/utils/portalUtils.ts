// src/utils/portalUtils.ts
import type { NavigateFunction } from 'react-router-dom';
import { PORTAL_CONSTANTS } from '../constants/portal';

/**
 * Utility to generate portal list path from current pathname
 */
export const getPortalListPath = (pathname: string): string => {
  const pathSegments = pathname.split('/').filter(Boolean);
  const portalsIndex = pathSegments.indexOf('portals');
  if (portalsIndex === -1) {
    console.warn('portalUtils: portals segment not found in path');
    return '/portals';
  }
  return '/' + pathSegments.slice(0, portalsIndex + 1).join('/');
};

/**
 * Navigate to portal list
 */
export const navigateToPortalList = (navigate: NavigateFunction, pathname: string): void => {
  const basePath = getPortalListPath(pathname);
  navigate(basePath);
};

/**
 * Navigate to create portal form
 */
export const navigateToPortalCreate = (navigate: NavigateFunction, pathname: string): void => {
  const basePath = getPortalListPath(pathname);
  navigate(`${basePath}/create`);
};

/**
 * Navigate to portal theme editor
 */
export const navigateToPortalTheme = (navigate: NavigateFunction, pathname: string, portalId: string): void => {
  const basePath = getPortalListPath(pathname);
  navigate(`${basePath}/${portalId}/theme`);
};

/**
 * Navigate to portal edit form
 */
export const navigateToPortalEdit = (navigate: NavigateFunction, pathname: string, portalId: string): void => {
  const basePath = getPortalListPath(pathname);
  navigate(`${basePath}/${portalId}/edit`);
};

/**
 * Determine current mode from pathname
 */
export const getPortalMode = (pathname: string): string => {
  const pathSegments = pathname.split('/').filter(Boolean);
  const portalsIndex = pathSegments.indexOf('portals');
  if (portalsIndex === -1) return PORTAL_CONSTANTS.MODES.LIST;

  const afterPortals = pathSegments.slice(portalsIndex + 1);

  if (afterPortals.length === 1 && afterPortals[0] === 'create') {
    return PORTAL_CONSTANTS.MODES.FORM;
  }

  if (afterPortals.length === 2 && afterPortals[1] === 'theme') {
    return PORTAL_CONSTANTS.MODES.THEME;
  }

  if (afterPortals.length === 2 && afterPortals[1] === 'edit') {
    return PORTAL_CONSTANTS.MODES.EDIT;
  }

  return PORTAL_CONSTANTS.MODES.LIST;
};

/**
 * Extract portal ID from pathname
 */
export const getPortalIdFromPath = (pathname: string): string | null => {
  const pathSegments = pathname.split('/').filter(Boolean);
  const portalsIndex = pathSegments.indexOf('portals');
  if (portalsIndex === -1) return null;

  const afterPortals = pathSegments.slice(portalsIndex + 1);
  if (afterPortals.length >= 2 && (afterPortals[1] === 'theme' || afterPortals[1] === 'edit')) {
    return afterPortals[0];
  }
  return null;
};

/**
 * Generate slug from name
 */
export const slugify = (s: string): string =>
  s
    .toLowerCase()
    .trim()
    .replace(/['"]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '')
    .slice(0, 64);
