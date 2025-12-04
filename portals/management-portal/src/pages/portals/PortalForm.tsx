// src/pages/portals/PortalForm.tsx
import React, { useCallback, useState, useEffect, useMemo } from 'react';
import {
  Box,
  Typography,
  Button,
  FormControl,
  CircularProgress,
  MenuItem,
} from '@mui/material';
import Grid from '@mui/material/GridLegacy';
import { Button as CustomButton } from '../../components/src/components/Button';
import { TextInput } from '../../components/src';
import { SimpleSelect } from '../../components/src/components/SimpleSelect';
import { slugify } from '../../utils/portalUtils';
import { PORTAL_CONSTANTS } from '../../constants/portal';
import type { CreatePortalPayload, UpdatePortalPayload } from '../../hooks/devportals';
import { useNotifications } from '../../context/NotificationContext';

interface PortalFormProps {
  onSubmit: (formData: CreatePortalPayload | UpdatePortalPayload) => Promise<void>;
  onCancel: () => void;
  isSubmitting?: boolean;
  initialData?: Partial<CreatePortalPayload>;
  isEdit?: boolean;
}

const initialFormData: CreatePortalPayload = {
  name: '',
  identifier: '',
  description: '',
  apiUrl: '',
  hostname: '',
  apiKey: '',
  headerKeyName: PORTAL_CONSTANTS.DEFAULT_HEADER_KEY_NAME,
  visibility: 'private',
};

const PortalForm: React.FC<PortalFormProps> = ({
  onSubmit,
  onCancel,
  isSubmitting = false,
  initialData,
  isEdit = false,
}) => {
  const [formData, setFormData] = useState<CreatePortalPayload>(() => ({
    ...initialFormData,
    ...initialData,
  }));

  const [submitStatus, setSubmitStatus] = useState<
    'idle' | 'pending' | 'success' | 'error'
  >('idle');
  const { showNotification } = useNotifications();

  useEffect(() => {
    setFormData({ ...initialFormData, ...initialData });
  }, [initialData]);

  const updateField = useCallback(
    <K extends keyof CreatePortalPayload>(
      field: K,
      value: CreatePortalPayload[K]
    ) => {
      setFormData((prev) => ({
        ...prev,
        [field]: value,
        // Auto-generate identifier from name only in add mode
        ...(field === 'name' && typeof value === 'string' && !isEdit
          ? {
              identifier: slugify(value),
            }
          : {}),
      }));
    },
    [isEdit]
  );

  const resetForm = useCallback(() => {
    setFormData({ ...initialFormData, ...initialData });
  }, [initialData]);

  const isValid = useMemo(() => {
    const apiKeyValid = isEdit && formData.apiKey === '*****' ? true : formData.apiKey.trim();
    const isValidUrl = (url: string) => {
      try {
        const parsed = new URL(url);
        return parsed.protocol === 'http:' || parsed.protocol === 'https:';
      } catch {
        return false;
      }
    };
    const apiUrlValid = formData.apiUrl.trim() && !formData.apiUrl.trim().endsWith('/') && isValidUrl(formData.apiUrl.trim());
    return !!(
      formData.name.trim() &&
      apiUrlValid &&
      formData.hostname.trim() &&
      apiKeyValid
    );
  }, [formData, isEdit]);

  const effectiveSubmitting = isSubmitting || submitStatus === 'pending';

  const handleSubmit = useCallback(async () => {
    if (!isValid || effectiveSubmitting) return;

    // Track status if not provided by parent
    if (!isSubmitting) {
      setSubmitStatus('pending');
    }

    try {
      let payload: UpdatePortalPayload = { ...formData };
      if (isEdit && payload.apiKey === '*****') {
        delete payload.apiKey;
      }
      await onSubmit(payload);

      if (!isSubmitting) {
        setSubmitStatus('success');
      }

      // For edit mode, show confirmation and navigate back
      if (isEdit) {
        showNotification('Developer Portal updated successfully.', 'success');
        resetForm();
        onCancel();
      }
    } catch (error) {
      if (!isSubmitting) {
        setSubmitStatus('error');
      }
      const errorMessage =
        error instanceof Error
          ? error.message
          : 'Failed to create Developer Portal.';
      showNotification(errorMessage, 'error');
    }
  }, [
    formData,
    isValid,
    isSubmitting,
    effectiveSubmitting,
    onSubmit,
    onCancel,
    resetForm,
    isEdit,
    showNotification,
  ]);

  return (
    <Box>
      {/* Header */}
      <Box sx={{ display: 'flex', alignItems: 'center', mb: 2 }}>
        <Button
          variant="text"
          onClick={onCancel}
          sx={{ mr: 2, minWidth: 'auto', px: 1 }}
          aria-label="Back to portal list"
        >
          ← Back
        </Button>
        <Typography variant="h5">
          {isEdit ? 'Edit Portal Details' : 'Add Your Portal Details'}
        </Typography>
      </Box>

      {/* Form Container */}
      <Box sx={{ width: 560, maxWidth: '100%', position: 'relative' }}>
        {/* Form Title */}
        <Typography variant="h5" sx={{ mb: 1 }}>
          {isEdit ? 'Edit Portal Details' : 'Add Your Portal Details'}
        </Typography>

        {/* Form Fields */}
        <Grid container spacing={2}>
          {/* Row 1: Name / Identifier */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="Portal name"
                placeholder="Provide a name for your portal"
                value={formData.name}
                onChange={(value) => updateField('name', value)}
                disabled={effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="Identifier"
                placeholder="Auto-generated from name"
                value={formData.identifier}
                onChange={(value) => updateField('identifier', value)}
                disabled={isEdit || effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>

          {/* Row 2: API URL / Hostname */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="API URL"
                placeholder="https://api.devportal.example.com"
                value={formData.apiUrl}
                onChange={(value) => updateField('apiUrl', value)}
                disabled={effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="Hostname"
                placeholder="devportal.example.com"
                value={formData.hostname}
                onChange={(value) => updateField('hostname', value)}
                disabled={effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>

          {/* Row 3: API Key / Header Key Name */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="API Key"
                placeholder="your-api-key-here"
                value={formData.apiKey}
                onChange={(value) => updateField('apiKey', value)}
                disabled={effectiveSubmitting}
                type="password"
                testId=""
              />
            </FormControl>
          </Grid>

          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <TextInput
                size="medium"
                label="Header Key Name"
                placeholder="x-wso2-api-key"
                value={formData.headerKeyName}
                onChange={(value) => updateField('headerKeyName', value)}
                disabled={effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>

          {/* Row 4: Visibility */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <Typography variant="body2" sx={{ mb: 1, fontWeight: 500 }}>
                Visibility
              </Typography>
              <SimpleSelect
                value={formData.visibility}
                onChange={(event) => updateField('visibility', event.target.value as 'public' | 'private')}
                disabled={effectiveSubmitting}
                testId="visibility-select"
                size="medium"
              >
                <MenuItem value="public">Public</MenuItem>
                <MenuItem value="private">Private</MenuItem>
              </SimpleSelect>
            </FormControl>
          </Grid>

          {/* Row 5: Description */}
          <Grid item xs={12}>
            <FormControl fullWidth>
              <TextInput
                label="Description"
                placeholder="Briefly describe your developer portal..."
                value={formData.description}
                onChange={(value) => updateField('description', value)}
                multiline
                disabled={effectiveSubmitting}
                testId=""
              />
            </FormControl>
          </Grid>
        </Grid>

        {/* Submit Button */}
        <CustomButton
          variant="contained"
          disabled={!isValid || effectiveSubmitting}
          onClick={handleSubmit}
          sx={{ mt: 2, minWidth: 140 }}
        >
          {effectiveSubmitting ? (
            <>
              <CircularProgress size={20} sx={{ mr: 1, color: 'inherit' }} />
              {isEdit ? 'Saving...' : 'Creating...'}
            </>
          ) : isEdit ? (
            'Save Changes'
          ) : (
            'Create and continue'
          )}
        </CustomButton>
      </Box>
    </Box>
  );
};

export default PortalForm;
