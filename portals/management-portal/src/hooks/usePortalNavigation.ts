// src/hooks/usePortalNavigation.ts
import { useParams, useNavigate, useLocation } from 'react-router-dom';
import { useMemo } from 'react';
import { PORTAL_CONSTANTS } from '../constants/portal';
import type { UsePortalNavigationReturn } from '../types/portal';

export const usePortalNavigation = (): UsePortalNavigationReturn => {
  const params = useParams();
  const navigate = useNavigate();
  const location = useLocation();

  const mode = useMemo(() => {
    const pathSegments = location.pathname.split('/').filter(Boolean);

    // Find the portals segment and check what comes after
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
  }, [location.pathname]);

  const selectedPortalId = params.portalId || null;

  const navigateToList = () => {
    const pathSegments = location.pathname.split('/').filter(Boolean);
    const portalsIndex = pathSegments.indexOf('portals');
    const basePath = '/' + pathSegments.slice(0, portalsIndex + 1).join('/');
    navigate(basePath);
  };

  const navigateToCreate = () => {
    const pathSegments = location.pathname.split('/').filter(Boolean);
    const portalsIndex = pathSegments.indexOf('portals');
    const basePath = '/' + pathSegments.slice(0, portalsIndex + 1).join('/');
    navigate(`${basePath}/create`);
  };

  const navigateToTheme = (portalId: string) => {
    const pathSegments = location.pathname.split('/').filter(Boolean);
    const portalsIndex = pathSegments.indexOf('portals');
    const basePath = '/' + pathSegments.slice(0, portalsIndex + 1).join('/');
    navigate(`${basePath}/${portalId}/theme`);
  };

  const navigateToEdit = (portalId: string) => {
    const pathSegments = location.pathname.split('/').filter(Boolean);
    const portalsIndex = pathSegments.indexOf('portals');
    const basePath = '/' + pathSegments.slice(0, portalsIndex + 1).join('/');
    navigate(`${basePath}/${portalId}/edit`);
  };

  return {
    mode,
    selectedPortalId,
    navigateToList,
    navigateToCreate,
    navigateToTheme,
    navigateToEdit,
  };
};