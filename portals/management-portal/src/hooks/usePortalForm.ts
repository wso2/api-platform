// src/hooks/usePortalForm.ts
import { useState, useCallback, useMemo, useEffect } from 'react';
import { PORTAL_CONSTANTS } from '../constants/portal';
import type { PortalFormData, UsePortalFormReturn } from '../types/portal';

const initialFormData: PortalFormData = {
  name: '',
  identifier: '',
  description: '',
  apiUrl: '',
  hostname: '',
  apiKey: '',
  headerKeyName: PORTAL_CONSTANTS.DEFAULT_HEADER_KEY_NAME,
};

const createFormState = (data?: PortalFormData): PortalFormData => ({
  ...initialFormData,
  ...data,
});

export const usePortalForm = (initialData?: PortalFormData): UsePortalFormReturn => {
  const [formData, setFormData] = useState<PortalFormData>(() => createFormState(initialData));

  const updateField = useCallback(<K extends keyof PortalFormData>(
    field: K,
    value: PortalFormData[K]
  ) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
      // Auto-generate identifier from name
      ...(field === 'name' && typeof value === 'string' ? {
        identifier: slugify(value)
      } : {}),
    }));
  }, []);

  const resetForm = useCallback(() => {
    setFormData(createFormState());
  }, []);

  useEffect(() => {
    setFormData(createFormState(initialData));
  }, [initialData]);

  const isValid = useMemo(() => {
    return !!(
      formData.name.trim() &&
      formData.apiUrl.trim() &&
      formData.hostname.trim() &&
      formData.apiKey.trim()
    );
  }, [formData]);

  return {
    formData,
    updateField,
    resetForm,
    isValid,
  };
};

// Utility function for slug generation
const slugify = (s: string): string =>
  s
    .toLowerCase()
    .trim()
    .replace(/['"]/g, '')
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/(^-|-$)/g, '')
    .slice(0, 64);