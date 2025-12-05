import React, { useEffect, useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Box,
  CircularProgress,
} from '@mui/material';
import { Button } from '../../../components/src/components/Button';
import { useApisContext } from '../../../context/ApiContext';
import { useNotifications } from '../../../context/NotificationContext';
import { PORTAL_CONSTANTS } from '../../../constants/portal';
import type { ApiSummary } from '../../../hooks/apis';
import { buildPublishPayload } from './mapper';
import ApiPublishForm from './ApiPublishForm';
import { firstServerUrl } from '../../../helpers/openApiHelpers';

interface PortalRef {
  uuid: string;
  name?: string;
}

interface Props {
  open: boolean;
  portal: PortalRef;
  api: ApiSummary;
  onClose: () => void;
  onPublish: (portalId: string, payload: any) => Promise<void>;
  publishing?: boolean;
}

const ApiPublishModal: React.FC<Props> = ({ open, portal, api, onClose, onPublish, publishing = false }) => {
  const { fetchGatewaysForApi } = useApisContext();
  const { showNotification } = useNotifications();

  const [showAdvanced, setShowAdvanced] = useState(false);
  const [gateways, setGateways] = useState<any[]>([]);
  const [loadingGateways, setLoadingGateways] = useState(false);

  const [formData, setFormData] = useState<any>({
    apiName: api?.name || '',
    productionURL: firstServerUrl(api) || '',
    sandboxURL: firstServerUrl(api) || '',
    apiDescription: api?.description || '',
    visibility: 'PUBLIC',
    technicalOwner: '',
    technicalOwnerEmail: '',
    businessOwner: '',
    businessOwnerEmail: '',
    labels: ['default'],
    subscriptionPolicies: [],
    tags: [],
    selectedDocumentIds: [],
  });

  const [newTag, setNewTag] = useState('');

  useEffect(() => {
    if (!open) return;
    let isMounted = true;
    
    setFormData((prev: any) => ({
      ...prev,
      apiName: api?.name || prev.apiName || '',
      apiDescription: api?.description || prev.apiDescription || '',
      productionURL: firstServerUrl(api) || prev.productionURL || '',
      sandboxURL: firstServerUrl(api) || prev.sandboxURL || '',
    }));

    (async () => {
      if (!api?.id) return;
      setLoadingGateways(true);
      try {
        const apiGateways = await fetchGatewaysForApi(api.id);
        if (isMounted) setGateways(apiGateways || []);
      } catch (err) {
        if (isMounted) setGateways([]);
      } finally {
        if (isMounted) setLoadingGateways(false);
      }
    })();

    return () => { isMounted = false; };
  }, [open, api, fetchGatewaysForApi]);

  const handleUrlChange = (type: 'production' | 'sandbox', url: string) => {
    setFormData((prev: any) => ({
      ...prev,
      [`${type}URL`]: url,
      ...(type === 'production' && !prev.sandboxURL ? { sandboxURL: url } : {}),
    }));
  };

  const handleCheckboxChange = (field: 'labels' | 'subscriptionPolicies' | 'selectedDocumentIds', value: string, checked: boolean) => {
    setFormData((prev: any) => ({
      ...prev,
      [field]: checked ? [...(prev[field] || []), value] : (prev[field] || []).filter((item: string) => item !== value),
    }));
  };

  const handleAddTag = () => {
    if (newTag.trim() && !formData.tags?.includes(newTag.trim())) {
      setFormData((prev: any) => ({ ...prev, tags: [...(prev.tags || []), newTag.trim()] }));
      setNewTag('');
    }
  };

  const handleRemoveTag = (tagToRemove: string) => {
    setFormData((prev: any) => ({ ...prev, tags: (prev.tags || []).filter((t: string) => t !== tagToRemove) }));
  };

  const handleSubmit = async () => {
    if (!portal?.uuid) {
      showNotification(PORTAL_CONSTANTS.MESSAGES.NO_PORTAL_SELECTED, 'error');
      return;
    }
    if (!formData.apiName || !formData.productionURL) {
      showNotification(PORTAL_CONSTANTS.MESSAGES.PROVIDE_API_NAME_AND_URL, 'error');
      return;
    }

    try {
      const payload = buildPublishPayload(formData, portal.uuid);
      await onPublish(portal.uuid, payload);
      showNotification(`Published to ${portal.name || 'portal'}`, 'success');
      onClose();
    } catch (err: any) {
      showNotification(err?.message || PORTAL_CONSTANTS.MESSAGES.PUBLISH_FAILED, 'error');
    }
  };

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="md">
      <DialogTitle>Add to {portal?.name || 'Developer Portal'}</DialogTitle>
      <DialogContent sx={{ pt: 1, pb: 2 }}>
        <ApiPublishForm
          formData={formData}
          setFormData={setFormData}
          showAdvanced={showAdvanced}
          setShowAdvanced={setShowAdvanced}
          gateways={gateways}
          loadingGateways={loadingGateways}
          newTag={newTag}
          setNewTag={setNewTag}
          handleAddTag={handleAddTag}
          handleRemoveTag={handleRemoveTag}
          handleCheckboxChange={handleCheckboxChange}
          handleUrlChange={handleUrlChange}
        />
      </DialogContent>

      <DialogActions sx={{ px: 3, py: 2 }}>
        <Button onClick={onClose} variant="outlined" disabled={publishing}>
          Cancel
        </Button>

        <Box sx={{ flex: 1 }} />

        <Button
          onClick={handleSubmit}
          variant="contained"
          disabled={publishing || !formData.apiName || !formData.productionURL}
        >
          {publishing ? (
            <Box display="flex" alignItems="center" gap={1}>
              <CircularProgress size={16} sx={{ color: 'white' }} />
              <span>Adding...</span>
            </Box>
          ) : (
            'Add'
          )}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ApiPublishModal;
