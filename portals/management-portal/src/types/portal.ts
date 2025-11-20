// src/types/portal.ts
import { PORTAL_CONSTANTS } from '../constants/portal';
import type { Portal } from '../hooks/devportals';

export interface PortalFormData {
  name: string;
  identifier: string;
  description: string;
  apiUrl: string;
  hostname: string;
  apiKey: string;
  headerKeyName: string;
}

export interface PortalUIModel {
  uuid: string;
  name: string;
  identifier: string;
  description: string;
  apiUrl: string;
  hostname: string;
  portalUrl: string;
  logoSrc?: string;
  logoAlt?: string;
  userAuthLabel: string;
  authStrategyLabel: string;
  visibilityLabel: string;
  isActive: boolean;
  createdAt?: string;
  updatedAt?: string;
}

export interface PortalCardProps {
  title: string;
  description: string;
  selected: boolean;
  onClick: () => void;
  logoSrc?: string;
  logoAlt?: string;
  portalUrl?: string;
  userAuthLabel?: string;
  authStrategyLabel?: string;
  visibilityLabel?: string;
  onEdit?: () => void;
  onActivate?: () => void;
  activating?: boolean;
  testId?: string;
  activateButtonText?: string;
}

export type PortalManagementProps = Record<string, never>;

export interface PortalListProps {
  portals: Portal[];
  loading: boolean;
  error: string | null;
  onPortalClick: (portalId: string) => void;
  onPortalActivate: (portalId: string) => void;
  onPortalEdit: (portalId: string) => void;
  onCreateNew: () => void;
}

export interface PortalFormProps {
  onSubmit: (formData: PortalFormData) => Promise<void>;
  onCancel: () => void;
  isSubmitting?: boolean;
  initialData?: PortalFormData;
  isEdit?: boolean;
}

export interface ThemeContainerProps {
  portalName?: string;
  onBack: () => void;
  onPublish?: () => void;
}

export interface PromoBannerProps {
  onPrimary?: () => void;
  imageSrc: string;
  imageAlt?: string;
}

export type ThemeSettingsPanelProps = Record<string, never>;

export interface PortalPreviewProps {
  type: typeof PORTAL_CONSTANTS.PORTAL_TYPES.PRIVATE | typeof PORTAL_CONSTANTS.PORTAL_TYPES.PUBLIC;
}

// API types
export type CreatePortalRequest = PortalFormData;

export interface ActivatePortalRequest {
  uuid: string;
}

export type PortalApiResponse = PortalUIModel;

// Error types
export class PortalError extends Error {
  code?: string;
  statusCode?: number;

  constructor(message: string, code?: string, statusCode?: number) {
    super(message);
    this.name = 'PortalError';
    this.code = code;
    this.statusCode = statusCode;
  }
}