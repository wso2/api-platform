import React, { useState, useEffect } from "react";
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Box,
  Typography,
  Stack,
  MenuItem,
  TextField,
  Checkbox as MUICheckbox,
  CircularProgress,
  Autocomplete,
} from "@mui/material";
import { Button } from "../../../components/src/components/Button";
import { Chip } from "../../../components/src/components/Chip";
import { useApisContext } from "../../../context/ApiContext";
import { useNotifications } from "../../../context/NotificationContext";
import type { ApiSummary } from "../../../hooks/apis";
import type { ApiGatewaySummary } from "../../../hooks/apis";
import type { ApiPublishPayload } from "../../../hooks/apiPublish";
import { buildPublishPayload } from "./mapper";

// Types
export interface ApiPublishFormData {
  // Basic fields
  apiName: string;

  // URLs (gateway URLs that can be selected or manually entered)
  productionURL: string;
  sandboxURL: string;

  // Advanced fields (matching payload)
  apiDescription?: string;
  visibility?: 'PUBLIC' | 'PRIVATE';
  technicalOwner?: string;
  technicalOwnerEmail?: string;
  businessOwner?: string;
  businessOwnerEmail?: string;
  labels?: string[];
  subscriptionPolicies?: string[];
  tags?: string[];
  selectedDocumentIds?: string[];
}


interface ApiPublishModalProps {
  open: boolean;
  portal: {
    uuid: string;
    name: string;
  };
  api: ApiSummary;
  onClose: () => void;
  onPublish: (portalId: string, payload: ApiPublishPayload) => Promise<void>;
  publishing?: boolean;
}

// Constants
const VISIBILITY_OPTIONS = [
  { value: 'PUBLIC', label: 'Public' },
  { value: 'PRIVATE', label: 'Private' },
] as const;

const AVAILABLE_LABELS = [
  'default', 'premium', 'internal', 'beta', 'deprecated', 'experimental'
];



// Subscription policies with descriptions
const SUBSCRIPTION_POLICIES = [
  {
    id: 'default',
    name: 'Default',
    description: 'Basic access with standard rate limits',
    price: 'Free',
    features: ['100 requests/hour', 'Basic support', 'Standard SLA']
  },
  {
    id: 'gold',
    name: 'Gold',
    description: 'Enhanced access for growing applications',
    price: '$29/month',
    features: ['1,000 requests/hour', 'Priority support', 'Enhanced SLA']
  },
  {
    id: 'platinum',
    name: 'Platinum',
    description: 'High-volume access for enterprise needs',
    price: '$99/month',
    features: ['10,000 requests/hour', '24/7 support', 'Premium SLA']
  },
  {
    id: 'enterprise',
    name: 'Enterprise',
    description: 'Unlimited access with dedicated support',
    price: 'Custom',
    features: ['Unlimited requests', 'Dedicated support', 'Custom SLA']
  },
  {
    id: 'free',
    name: 'Free',
    description: 'Limited access for testing and development',
    price: 'Free',
    features: ['50 requests/hour', 'Community support', 'Basic SLA']
  },
  {
    id: 'developer',
    name: 'Developer',
    description: 'Perfect for development and testing',
    price: '$9/month',
    features: ['500 requests/hour', 'Developer support', 'Standard SLA']
  }
];

// Dummy document data
const DUMMY_DOCUMENTS = [
  { id: 'api-ref-v1', name: 'API Reference v1.0.pdf', size: '2.3 MB' },
  { id: 'getting-started', name: 'Getting Started Guide.pdf', size: '1.1 MB' },
  { id: 'changelog', name: 'Changelog v1.0.md', size: '45 KB' },
  { id: 'examples', name: 'Code Examples.zip', size: '892 KB' },
];

// Utility functions
const getGatewayUrl = (gateway: ApiGatewaySummary): string => {
  const vhost = gateway.vhost || `${gateway.name.toLowerCase()}.api.example.com`;
  return vhost.startsWith('http') ? vhost : `https://${vhost}`;
};


const generatePublishDefaults = (api: ApiSummary): ApiPublishFormData => {

  return {
    apiName: api.name,
    productionURL: '',
    sandboxURL: '',
    apiDescription: api.description || '',
    visibility: 'PUBLIC',
    technicalOwner: '',
    technicalOwnerEmail: '',
    businessOwner: '',
    businessOwnerEmail: '',
    labels: ['default'],
    subscriptionPolicies: [],
    tags: [],
    selectedDocumentIds: ['api-ref-v1', 'changelog'],
  };
};

const ApiPublishModal: React.FC<ApiPublishModalProps> = ({
  open,
  portal,
  api,
  onClose,
  onPublish,
  publishing = false,
}) => {
  const { fetchGatewaysForApi } = useApisContext();
  const { showNotification } = useNotifications();

  // State
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [gateways, setGateways] = useState<ApiGatewaySummary[]>([]);
  const [loadingGateways, setLoadingGateways] = useState(false);
  const [formData, setFormData] = useState<ApiPublishFormData>(generatePublishDefaults(api));
  const [newTag, setNewTag] = useState('');

  // Load gateways when modal opens
  useEffect(() => {
    if (open && api?.id) {
      const loadGateways = async () => {
        setLoadingGateways(true);
        try {
          const apiGateways = await fetchGatewaysForApi(api.id);
          setGateways(apiGateways);
        } catch (error) {
          console.error('Failed to load gateways:', error);
        } finally {
          setLoadingGateways(false);
        }
      };

      loadGateways();
    }
  }, [open, api?.id, fetchGatewaysForApi]);

  // Reset form when modal opens
  useEffect(() => {
    if (open) {
      setFormData(generatePublishDefaults(api));
      setShowAdvanced(false);
      setNewTag('');
    }
  }, [open, api]);

  // Handle URL changes (either selected from dropdown or manually entered)
  const handleUrlChange = (type: 'production' | 'sandbox', url: string) => {
    setFormData(prev => ({
      ...prev,
      [`${type}URL`]: url,
      // Auto-set sandbox URL when production changes, if sandbox is empty
      ...(type === 'production' && !prev.sandboxURL ? {
        sandboxURL: url
      } : {})
    }));
  };

  // Handle checkbox changes
  const handleCheckboxChange = (field: 'labels' | 'subscriptionPolicies' | 'selectedDocumentIds', value: string, checked: boolean) => {
    setFormData(prev => ({
      ...prev,
      [field]: checked
        ? [...(prev[field] || []), value]
        : (prev[field] || []).filter(item => item !== value)
    }));
  };

  // Handle tag addition
  const handleAddTag = () => {
    if (newTag.trim() && !formData.tags?.includes(newTag.trim())) {
      setFormData(prev => ({
        ...prev,
        tags: [...(prev.tags || []), newTag.trim()]
      }));
      setNewTag('');
    }
  };

  // Handle tag removal
  const handleRemoveTag = (tagToRemove: string) => {
    setFormData(prev => ({
      ...prev,
      tags: (prev.tags || []).filter(tag => tag !== tagToRemove)
    }));
  };

  // Handle form submission
  const handleSubmit = async () => {
    try {
      const payload: ApiPublishPayload = buildPublishPayload(formData as any, portal.uuid);
      await onPublish(portal.uuid, payload);
      showNotification('API published successfully!', 'success');
      onClose();
    } catch (error) {
      console.error('Failed to publish:', error);
      
      // Extract error message from the error object
      let errorMessage = 'Failed to publish API. Please try again.';
      
      if (error && typeof error === 'object') {
        if ('message' in error && typeof error.message === 'string') {
          errorMessage = error.message;
        } else if ('response' in error && error.response && typeof error.response === 'object') {
          if ('data' in error.response && error.response.data && typeof error.response.data === 'object') {
            if ('message' in error.response.data && typeof error.response.data.message === 'string') {
              errorMessage = error.response.data.message;
            } else if ('description' in error.response.data && typeof error.response.data.description === 'string') {
              errorMessage = error.response.data.description;
            }
          }
        }
      }
      
      showNotification(errorMessage, 'error');
    }
  };

  // Gateway options for dropdowns (showing name + URL)
  const gatewayOptions = gateways.map(gateway => ({
    label: `${gateway.displayName || gateway.name} (${getGatewayUrl(gateway)})`,
    value: getGatewayUrl(gateway)
  }));

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      PaperProps={{
        sx: { 
          maxHeight: '80vh',
          borderRadius: 3,
          width: '650px',
          maxWidth: '90vw',
          p:3
        }
      }}
    >
      <DialogTitle sx={{ fontWeight: 700, fontSize: 20, pb: 1 }}>
        Publish API
      </DialogTitle>

      <DialogContent sx={{ pt: 1, pb: 2 }}>
        <Stack spacing={2.5}>
          {/* Basic API Information */}
          <Box>
            <Typography variant="subtitle1" fontWeight={600} color="text.primary" mb={2}>
              API Information
            </Typography>
            <Stack spacing={2}>
              <TextField
                label="API Name"
                value={formData.apiName}
                onChange={(e) => setFormData(prev => ({ ...prev, apiName: e.target.value }))}
                fullWidth
                required
                variant="outlined"
                placeholder="My Sample API"
                helperText="This name will be displayed in the developer portal"
              />

              <TextField
                label="API Description"
                value={formData.apiDescription || ''}
                onChange={(e) => setFormData(prev => ({ ...prev, apiDescription: e.target.value }))}
                fullWidth
                multiline
                rows={2}
                variant="outlined"
                placeholder="A RESTful API for managing user accounts and transactions"
                helperText="Help developers understand the purpose and capabilities of your API"
              />

              <TextField
                select
                label="Access Visibility"
                value={formData.visibility || 'PUBLIC'}
                onChange={(e) => setFormData(prev => ({ ...prev, visibility: e.target.value as any }))}
                fullWidth
                variant="outlined"
                helperText="Control who can discover and access your API"
              >
                {VISIBILITY_OPTIONS.map(option => (
                  <MenuItem key={option.value} value={option.value}>
                    <Box display="flex" alignItems="center" gap={1}>
                      <Box component="span">
                        {option.value === 'PUBLIC' ? '🌍' : '🔒'}
                      </Box>
                      <Typography variant="body2">
                        {option.label}
                      </Typography>
                    </Box>
                  </MenuItem>
                ))}
              </TextField>
            </Stack>
          </Box>

          {/* Endpoints */}
          <Box>
            <Typography variant="subtitle1" fontWeight={600} color="text.primary" mb={1.5}>
              Endpoints
            </Typography>
            <Stack spacing={2.5}>
              {/* Production */}
              <Box>
                <Typography variant="body2" fontWeight={600} color="text.primary" mb={1}>
                  Production Gateway URL
                </Typography>
                <Autocomplete
                  freeSolo
                  options={gatewayOptions}
                  getOptionLabel={(option) => typeof option === 'string' ? option : option.label}
                  inputValue={formData.productionURL}
                  onInputChange={(_, newInputValue) => {
                    const selectedOption = gatewayOptions.find(option => 
                      typeof option === 'object' && option.label === newInputValue
                    );
                    const url = selectedOption ? selectedOption.value : newInputValue || '';
                    handleUrlChange('production', url);
                  }}
                  renderInput={(params) => (
                    <TextField
                      {...params}
                      placeholder="https://prod.example.com/api"
                      fullWidth
                      disabled={loadingGateways}
                      helperText="Select a configured gateway or enter an external URL for live API traffic"
                    />
                  )}
                  renderOption={(props, option) => {
                    const { key, ...otherProps } = props;
                    return (
                      <li key={key} {...otherProps}>
                        <Box display="flex" alignItems="center" gap={2} width="100%" py={1}>
                          <Box
                            sx={{
                              width: 36,
                              height: 36,
                              borderRadius: 1,
                              backgroundColor: 'primary.main',
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center',
                              color: 'white',
                              fontWeight: 600,
                              fontSize: '0.8rem'
                            }}
                          >
                            {option.label.split(' ')[0]?.substring(0, 2).toUpperCase() || 'GW'}
                          </Box>
                          <Box flex={1}>
                              <Typography variant="body2" fontWeight={500} color="text.primary">
                                {option.label.split('(')[0].trim()}
                              </Typography>
                              <Typography variant="body2" color="text.secondary" fontWeight={400}>
                                {option.value}
                              </Typography>
                          </Box>
                        </Box>
                      </li>
                    );
                  }}
                />
              </Box>

              {/* Sandbox - Only show in advanced mode */}
              {showAdvanced && (
                <Box>
                  <Typography variant="body2" fontWeight={600} color="text.primary" mb={1}>
                    Sandbox Gateway URL
                  </Typography>
                  <Autocomplete
                    freeSolo
                    options={gatewayOptions}
                    getOptionLabel={(option) => typeof option === 'string' ? option : option.label}
                    inputValue={formData.sandboxURL}
                    onInputChange={(_, newInputValue) => {
                      const selectedOption = gatewayOptions.find(option => 
                        typeof option === 'object' && option.label === newInputValue
                      );
                      const url = selectedOption ? selectedOption.value : newInputValue || '';
                      handleUrlChange('sandbox', url);
                    }}
                    renderInput={(params) => (
                      <TextField
                        {...params}
                        placeholder="https://sandbox.example.com/api"
                        fullWidth
                        disabled={loadingGateways}
                        helperText="Used for testing and exploration by developers"
                      />
                    )}
                    renderOption={(props, option) => {
                      const { key, ...otherProps } = props;
                      return (
                        <li key={key} {...otherProps}>
                          <Box display="flex" alignItems="center" gap={2} width="100%" py={1}>
                            <Box
                              sx={{
                                width: 36,
                                height: 36,
                                borderRadius: 1,
                                backgroundColor: 'primary.main',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center',
                                color: 'white',
                                fontWeight: 600,
                                fontSize: '0.8rem'
                              }}
                            >
                              {option.label.split(' ')[0]?.substring(0, 2).toUpperCase() || 'GW'}
                            </Box>
                            <Box flex={1}>
                              <Typography variant="body2" fontWeight={500} color="text.primary">
                                {option.label.split('(')[0].trim()}
                              </Typography>
                              <Typography variant="body2" color="text.secondary" fontWeight={400}>
                                {option.value}
                              </Typography>
                            </Box>
                          </Box>
                        </li>
                      );
                    }}
                  />
                </Box>
              )}
            </Stack>
          </Box>

          {/* Advanced Options Toggle */}
          <Box sx={{ textAlign: 'center' }}>
            <Button
              variant="outlined"
              onClick={() => setShowAdvanced(!showAdvanced)}
              fullWidth
              sx={{
                py: 1.5,
                borderRadius: 2,
                textTransform: 'none',
                fontWeight: 500,
                borderColor: 'divider',
                color: 'text.secondary',
                '&:hover': {
                  borderColor: 'primary.main',
                  backgroundColor: 'primary.50',
                  color: 'primary.main'
                }
              }}
              startIcon={
                <Box sx={{ fontSize: '1.2rem', lineHeight: 1 }}>
                  {showAdvanced ? '▲' : '▼'}
                </Box>
              }
            >
              {showAdvanced ? 'Hide Additional Settings' : 'Show Additional Settings'}
            </Button>
            {!showAdvanced && (
              <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
                Configure sandbox URL, ownership, labels, policies, and documentation
              </Typography>
            )}
          </Box>

          {/* Advanced Options */}
          {showAdvanced && (
            <>
              <Box sx={{ mt: 2 }}>


                <Stack spacing={3}>
                  {/* Owners */}
                  <Box>
                    <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">
                      Contacts
                    </Typography>
                    <Stack spacing={2}>
                      <Stack direction="row" spacing={2}>
                        <TextField
                          label="Technical Contact"
                          value={formData.technicalOwner || ''}
                          onChange={(e) => setFormData(prev => ({ ...prev, technicalOwner: e.target.value }))}
                          fullWidth
                          placeholder="John Doe"
                          helperText="Primary technical contact for this API"
                        />
                        <TextField
                          label="Technical Email"
                          value={formData.technicalOwnerEmail || ''}
                          onChange={(e) => setFormData(prev => ({ ...prev, technicalOwnerEmail: e.target.value }))}
                          fullWidth
                          type="email"
                          placeholder="john.doe@company.com"
                          helperText="Email for technical support and issues"
                        />
                      </Stack>
                      <Stack direction="row" spacing={2}>
                        <TextField
                          label="Business Contact"
                          value={formData.businessOwner || ''}
                          onChange={(e) => setFormData(prev => ({ ...prev, businessOwner: e.target.value }))}
                          fullWidth
                          placeholder="Jane Smith"
                          helperText="Business stakeholder for this API"
                        />
                        <TextField
                          label="Business Email"
                          value={formData.businessOwnerEmail || ''}
                          onChange={(e) => setFormData(prev => ({ ...prev, businessOwnerEmail: e.target.value }))}
                          fullWidth
                          type="email"
                          placeholder="jane.smith@company.com"
                          helperText="Contact for business-related inquiries"
                        />
                      </Stack>
                    </Stack>
                  </Box>

                  {/* Labels & Policies */}
                  <Box>
                    <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">
                      Labels & Policies
                    </Typography>
                    <Stack spacing={2.5}>
                      <Box>
                        <Typography variant="body2" fontWeight={500} mb={1.5} color="text.primary">
                          Labels
                        </Typography>
                        <Box display="flex" flexWrap="wrap" gap={1}>
                          {AVAILABLE_LABELS.map(label => (
                            <Chip
                              key={label}
                              label={label}
                              onClick={() => {
                                const isSelected = formData.labels?.includes(label) || false;
                                handleCheckboxChange('labels', label, !isSelected);
                              }}
                              variant={formData.labels?.includes(label) ? 'filled' : 'outlined'}
                              color={formData.labels?.includes(label) ? 'primary' : 'default'}
                              size="medium"
                              sx={{ 
                                cursor: 'pointer',
                                '&:hover': { 
                                  transform: 'scale(1.05)',
                                  boxShadow: 1
                                },
                                transition: 'all 0.2s'
                              }}
                            />
                          ))}
                        </Box>
                      </Box>

                      <Box>
                        <Typography variant="body2" fontWeight={500} mb={1.5} color="text.primary">
                          Subscription Policies
                        </Typography>
                        <Box display="grid" gridTemplateColumns={{ xs: '1fr', sm: '1fr 1fr' }} gap={2}>
                          {SUBSCRIPTION_POLICIES.map(policy => (
                            <Box 
                              key={policy.id}
                              sx={{
                                p: 2.5,
                                border: '2px solid',
                                borderColor: formData.subscriptionPolicies?.includes(policy.name) ? 'primary.main' : 'grey.200',
                                borderRadius: 2,
                                backgroundColor: formData.subscriptionPolicies?.includes(policy.name) ? 'primary.50' : 'white',
                                cursor: 'pointer',
                                transition: 'all 0.2s ease-in-out',
                                '&:hover': {
                                  borderColor: 'primary.main',
                                  backgroundColor: 'primary.50',
                                  transform: 'translateY(-2px)',
                                  boxShadow: 2
                                }
                              }}
                              onClick={() => {
                                const isSelected = formData.subscriptionPolicies?.includes(policy.name) || false;
                                handleCheckboxChange('subscriptionPolicies', policy.name, !isSelected);
                              }}
                            >
                              <Box display="flex" alignItems="flex-start" gap={1.5}>
                                <MUICheckbox
                                  checked={formData.subscriptionPolicies?.includes(policy.name) || false}
                                  onChange={(e) => {
                                    e.stopPropagation();
                                    handleCheckboxChange('subscriptionPolicies', policy.name, e.target.checked);
                                  }}
                                  size="small"
                                  color="primary"
                                />
                                <Box flex={1}>
                                  <Typography variant="subtitle2" fontWeight={600} color="text.primary" mb={0.5}>
                                    {policy.name}
                                  </Typography>
                                  <Typography variant="body2" color="text.secondary" sx={{ lineHeight: 1.4 }}>
                                    {policy.description}
                                  </Typography>
                                </Box>
                              </Box>
                            </Box>
                          ))}
                        </Box>
                      </Box>
                    </Stack>
                  </Box>

                  {/* Tags */}
                  <Box>
                    <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">
                      Tags
                    </Typography>
                    <Box>
                      <Box display="flex" gap={1} mb={2}>
                        <TextField
                          label="Add Tag"
                          value={newTag}
                          onChange={(e) => setNewTag(e.target.value)}
                          onKeyPress={(e) => {
                            if (e.key === 'Enter') {
                              e.preventDefault();
                              handleAddTag();
                            }
                          }}
                          size="small"
                          placeholder="e.g., mobile, finance, v2"
                          sx={{ minWidth: 200 }}
                        />
                        <Button 
                          onClick={handleAddTag} 
                          disabled={!newTag.trim()}
                          variant="outlined"
                          size="small"
                          sx={{ 
                            minWidth: 60,
                            textTransform: 'none'
                          }}
                        >
                          Add
                        </Button>
                      </Box>
                      
                      <Box>
                        <Typography variant="body2" color="text.secondary" mb={1}>
                          Added Tags:
                        </Typography>
                        {formData.tags && formData.tags.length > 0 ? (
                          <Box 
                            sx={{
                              display: 'flex', 
                              flexWrap: 'wrap', 
                              gap: 1.5,
                              p: 3,
                              backgroundColor: 'primary.50',
                              borderRadius: 2,
                              border: '1px solid',
                              borderColor: 'primary.200',
                              minHeight: 60
                            }}
                          >
                            {formData.tags?.map(tag => (
                              <Chip
                                key={tag}
                                label={tag}
                                onDelete={() => handleRemoveTag(tag)}
                                size="medium"
                                variant="filled"
                                color="primary"
                                sx={{
                                  height: 32,
                                  fontSize: '0.875rem',
                                  fontWeight: 500,
                                  '& .MuiChip-deleteIcon': {
                                    fontSize: '18px',
                                    '&:hover': { 
                                      color: 'error.main',
                                      backgroundColor: 'rgba(255, 255, 255, 0.2)'
                                    }
                                  },
                                  '&:hover': {
                                    backgroundColor: 'primary.dark'
                                  }
                                }}
                              />
                            ))}
                          </Box>
                        ) : (
                          <Box 
                            sx={{
                              p: 3,
                              backgroundColor: 'grey.50',
                              borderRadius: 2,
                              border: '1px dashed',
                              borderColor: 'grey.300',
                              textAlign: 'center',
                              minHeight: 60,
                              display: 'flex',
                              alignItems: 'center',
                              justifyContent: 'center'
                            }}
                          >
                            <Typography variant="body2" color="text.secondary">
                              No tags added yet. Add tags to help categorize your API.
                            </Typography>
                          </Box>
                        )}
                      </Box>
                    </Box>
                  </Box>

                  {/* Documents */}
                  <Box>
                    <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">
                      Documentation
                    </Typography>
                    <Stack spacing={1.5}>
                      {DUMMY_DOCUMENTS.map(doc => (
                        <Box 
                          key={doc.id} 
                          sx={{
                            p: 2,
                            border: '1px solid',
                            borderColor: formData.selectedDocumentIds?.includes(doc.id) ? 'primary.main' : 'grey.200',
                            borderRadius: 1.5,
                            backgroundColor: formData.selectedDocumentIds?.includes(doc.id) ? 'primary.50' : 'white',
                            cursor: 'pointer',
                            transition: 'all 0.2s',
                            '&:hover': {
                              borderColor: 'primary.main',
                              backgroundColor: 'primary.50',
                              transform: 'translateX(4px)'
                            }
                          }}
                          onClick={() => {
                            const isSelected = formData.selectedDocumentIds?.includes(doc.id) || false;
                            handleCheckboxChange('selectedDocumentIds', doc.id, !isSelected);
                          }}
                        >
                          <Box display="flex" alignItems="center" gap={2}>
                            <MUICheckbox
                              checked={formData.selectedDocumentIds?.includes(doc.id) || false}
                              onChange={(e) => {
                                e.stopPropagation();
                                handleCheckboxChange('selectedDocumentIds', doc.id, e.target.checked);
                              }}
                              color="primary"
                            />
                            <Box
                              sx={{
                                width: 32,
                                height: 32,
                                borderRadius: 1,
                                backgroundColor: 'grey.100',
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'center'
                              }}
                            >
                              📄
                            </Box>
                            <Box flex={1}>
                              <Typography variant="body1" fontWeight={500} color="text.primary">
                                {doc.name}
                              </Typography>
                              <Typography variant="body2" color="text.secondary">
                                {doc.size}
                              </Typography>
                            </Box>
                          </Box>
                        </Box>
                      ))}
                    </Stack>
                  </Box>
                </Stack>
              </Box>
            </>
          )}
        </Stack>
      </DialogContent>

      <DialogActions 
        sx={{ 
          px: 3, 
          py: 2, 
          gap: 2,
          position: 'sticky',
          bottom: 0,
          backgroundColor: 'background.paper',
          borderTop: '1px solid',
          borderColor: 'divider'
        }}
      >
        <Button 
          onClick={onClose} 
          disabled={publishing}
          variant="outlined"
          sx={{ 
            textTransform: 'none'
          }}
        >
          Cancel
        </Button>
        
        <Box sx={{ flex: 1 }} />
        
        {!formData.apiName && (
          <Typography variant="body2" color="error.main" sx={{ fontSize: '0.8rem' }}>
            API Name is required
          </Typography>
        )}
        
        <Button
          onClick={handleSubmit}
          disabled={publishing || !formData.apiName}
          variant="contained"
          sx={{
            textTransform: 'none',
            fontWeight: 600,
            minWidth: 100
          }}
        >
          {publishing ? (
            <Box display="flex" alignItems="center" gap={1}>
              <CircularProgress size={16} sx={{ color: 'white' }} />
              <span>Publishing...</span>
            </Box>
          ) : (
            'Publish'
          )}
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default ApiPublishModal;