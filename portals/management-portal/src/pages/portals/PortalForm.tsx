// src/pages/portals/PortalForm.tsx
import React, { useCallback } from 'react';
import { Box, Typography, Button, Snackbar, Alert, FormControl, CircularProgress } from '@mui/material';
import Grid from '@mui/material/GridLegacy';
import { Button as CustomButton } from '../../components/src/components/Button';
import { TextInput } from '../../components/src';
import { usePortalForm } from '../../hooks/usePortalForm';
import { useFormStatus } from '../../hooks/useFormStatus';
import type { PortalFormProps } from '../../types/portal';

const PortalForm: React.FC<PortalFormProps> = ({
  onSubmit,
  onCancel,
  isSubmitting = false,
  initialData,
  isEdit = false,
}) => {
  const { formData, updateField, resetForm, isValid } = usePortalForm(initialData);
  const [snackbar, setSnackbar] = React.useState<{
    open: boolean;
    message: string;
    severity: 'success' | 'error';
  }>({
    open: false,
    message: '',
    severity: 'success',
  });
  // use shared hook to track async submit status when parent doesn't provide isSubmitting
  const { status: submitStatus, run: runSubmit } = useFormStatus();
  // Fallback boolean for UI
  const effectiveSubmitting = isSubmitting || submitStatus === 'pending';

  const handleSubmit = useCallback(async () => {
    if (!isValid || effectiveSubmitting) return;

    try {
      if (!isSubmitting) {
        // run via the form-status hook so it tracks pending/error/success
        await runSubmit(async () => await onSubmit(formData));
      } else {
        await onSubmit(formData);
      }
      // For creation, PortalManagement will handle navigation and snackbar
      // For edit, reset form and navigate back
      if (isEdit) {
        setSnackbar({
          open: true,
          message: 'Developer Portal updated successfully.',
          severity: 'success',
        });
        resetForm();
        onCancel(); // Navigate back to list
      }
      // For creation, don't navigate here; PortalManagement handles it
    } catch (error) {
      setSnackbar({
        open: true,
        message: error instanceof Error ? error.message : 'Failed to create Developer Portal.',
        severity: 'error',
      });
    }
    finally {
      // no-op; hook manages its own state
    }
  }, [formData, isValid, isSubmitting, onSubmit, onCancel, resetForm, isEdit, effectiveSubmitting, runSubmit]);

  const handleCloseSnackbar = useCallback(() => {
    setSnackbar(prev => ({ ...prev, open: false }));
  }, []);

  return (
    <Box>
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
          Add Your Portal Details
        </Typography>
      </Box>

      <Box sx={{ width: 560, maxWidth: '100%', position: 'relative' }}>
        <Box>
          <Typography variant="h5" sx={{ mb: 1 }}>
            Add Your Portal Details
          </Typography>

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
                  disabled={isSubmitting}
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
                  disabled={isSubmitting}
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
                  disabled={isSubmitting}
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
                  testId=""
                />
              </FormControl>
            </Grid>

            {/* Row 4: API Key / Header Key Name */}
            <Grid item xs={12} md={6}>
              <FormControl fullWidth>
                <TextInput
                  size="medium"
                  label="API Key"
                  placeholder="your-api-key-here"
                  value={formData.apiKey}
                  onChange={(value) => updateField('apiKey', value)}
                  disabled={isSubmitting}
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
                  disabled={isSubmitting}
                  testId=""
                />
              </FormControl>
            </Grid>

            {/* Row 4: Description */}
            <Grid item xs={12}>
              <FormControl fullWidth>
                <TextInput
                  label="Description"
                  placeholder="Briefly describe your developer portal..."
                  value={formData.description}
                  onChange={(value) => updateField('description', value)}
                  multiline
                  disabled={isSubmitting}
                  testId=""
                />
              </FormControl>
            </Grid>
          </Grid>

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
            ) : (
              'Create and continue'
            )}
          </CustomButton>
        </Box>
      </Box>

      <Snackbar
        open={snackbar.open}
        autoHideDuration={4000}
        onClose={handleCloseSnackbar}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert
          onClose={handleCloseSnackbar}
          severity={snackbar.severity}
          sx={{ width: '100%' }}
        >
          {snackbar.message}
        </Alert>
      </Snackbar>
    </Box>
  );
};

export default PortalForm;