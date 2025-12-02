import React from 'react';
import {
  Box,
  Typography,
  Stack,
  MenuItem,
  TextField,
  Checkbox as MUICheckbox,
  Autocomplete,
} from '@mui/material';
import { Button } from '../../../components/src/components/Button';
import { Chip } from '../../../components/src/components/Chip';

export const AVAILABLE_LABELS = [
  'default', 'premium', 'internal', 'beta', 'deprecated', 'experimental'
];

export const SUBSCRIPTION_POLICIES = [
  { id: 'default', name: 'Default', description: 'Basic access with standard rate limits' },
  { id: 'gold', name: 'Gold', description: 'Enhanced access for growing applications' },
  { id: 'platinum', name: 'Platinum', description: 'High-volume access for enterprise needs' },
];

export const DUMMY_DOCUMENTS = [
  { id: 'api-ref-v1', name: 'API Reference v1.0.pdf', size: '2.3 MB' },
  { id: 'getting-started', name: 'Getting Started Guide.pdf', size: '1.1 MB' },
];

interface Gateway {
  name: string;
  displayName?: string;
  vhost?: string;
}

interface FormData {
  apiName: string;
  productionURL: string;
  sandboxURL: string;
  apiDescription: string;
  visibility: 'PUBLIC' | 'PRIVATE';
  technicalOwner: string;
  technicalOwnerEmail: string;
  businessOwner: string;
  businessOwnerEmail: string;
  labels: string[];
  subscriptionPolicies: string[];
  tags: string[];
  selectedDocumentIds: string[];
}

interface Props {
  formData: FormData;
  setFormData: (v: FormData | ((prev: FormData) => FormData)) => void;
  showAdvanced: boolean;
  setShowAdvanced: (v: boolean) => void;
  gateways: Gateway[];
  loadingGateways: boolean;
  newTag: string;
  setNewTag: (v: string) => void;
  handleAddTag: () => void;
  handleRemoveTag: (tag: string) => void;
  handleCheckboxChange: (field: 'labels' | 'subscriptionPolicies' | 'selectedDocumentIds', value: string, checked: boolean) => void;
  handleUrlChange: (type: 'production' | 'sandbox', url: string) => void;
}

const getGatewayUrl = (gateway: Gateway): string => {
  const vhost = gateway.vhost || `${gateway.name?.toLowerCase()}.api.example.com`;
  return vhost.startsWith('http') ? vhost : `https://${vhost}`;
};

const ApiPublishForm: React.FC<Props> = ({
  formData,
  setFormData,
  showAdvanced,
  setShowAdvanced,
  gateways,
  loadingGateways,
  newTag,
  setNewTag,
  handleAddTag,
  handleRemoveTag,
  handleCheckboxChange,
  handleUrlChange,
}) => {
  const gatewayOptions = gateways.map((gateway: any) => ({ label: `${gateway.displayName || gateway.name} (${getGatewayUrl(gateway)})`, value: getGatewayUrl(gateway) }));

  return (
    <Stack spacing={2}>
      <Box>
        <Typography variant="subtitle1" fontWeight={600} color="text.primary" mb={2}>
          API Information
        </Typography>
        <Stack spacing={2}>
          <TextField
            label="API Name"
            value={formData.apiName}
            onChange={(e) => setFormData((prev: any) => ({ ...prev, apiName: e.target.value }))}
            fullWidth
            required
            variant="outlined"
            placeholder="My Sample API"
            helperText="This name will be displayed in the developer portal"
          />

          <TextField
            label="API Description"
            value={formData.apiDescription || ''}
            onChange={(e) => setFormData((prev: any) => ({ ...prev, apiDescription: e.target.value }))}
            fullWidth
            multiline
            rows={2}
            variant="outlined"
            placeholder="A RESTful API for ..."
            helperText="Help developers understand the purpose and capabilities of your API"
          />

          <TextField
            select
            label="Access Visibility"
            value={formData.visibility || 'PUBLIC'}
            onChange={(e) => setFormData((prev: any) => ({ ...prev, visibility: e.target.value }))}
            fullWidth
            variant="outlined"
            helperText="Control who can discover and access your API"
          >
            <MenuItem value="PUBLIC">
              <Box display="flex" alignItems="center" gap={1}>
                <span>🌍</span>
                <Typography variant="body2">Public</Typography>
              </Box>
            </MenuItem>
            <MenuItem value="PRIVATE">
              <Box display="flex" alignItems="center" gap={1}>
                <span>🔒</span>
                <Typography variant="body2">Private</Typography>
              </Box>
            </MenuItem>
          </TextField>
        </Stack>
      </Box>

      <Box>
        <Typography variant="subtitle1" fontWeight={600} color="text.primary" mb={1.5}>
          Endpoints
        </Typography>
        <Stack spacing={2.5}>
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
                const selectedOption = gatewayOptions.find(option => typeof option === 'object' && option.label === newInputValue);
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
            />
          </Box>

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
                  const selectedOption = gatewayOptions.find(option => typeof option === 'object' && option.label === newInputValue);
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
              />
            </Box>
          )}
        </Stack>
      </Box>

      <Box sx={{ textAlign: 'center' }}>
        <Button
          variant="outlined"
          onClick={() => setShowAdvanced(!showAdvanced)}
          fullWidth
          sx={{ py: 1.5, borderRadius: 2, textTransform: 'none', fontWeight: 500 }}
        >
          {showAdvanced ? 'Hide Additional Settings' : 'Show Additional Settings'}
        </Button>
        {!showAdvanced && (
          <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
            Configure sandbox URL, ownership, labels, policies, and documentation
          </Typography>
        )}
      </Box>

      {showAdvanced && (
        <Box sx={{ mt: 2 }}>
          <Stack spacing={3}>
            <Box>
              <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">
                Contacts
              </Typography>
              <Stack spacing={2}>
                <Stack direction="row" spacing={2}>
                  <TextField
                    label="Technical Contact"
                    value={formData.technicalOwner || ''}
                    onChange={(e) => setFormData((prev: any) => ({ ...prev, technicalOwner: e.target.value }))}
                    fullWidth
                    placeholder="John Doe"
                    helperText="Primary technical contact for this API"
                  />
                  <TextField
                    label="Technical Email"
                    value={formData.technicalOwnerEmail || ''}
                    onChange={(e) => setFormData((prev: any) => ({ ...prev, technicalOwnerEmail: e.target.value }))}
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
                    onChange={(e) => setFormData((prev: any) => ({ ...prev, businessOwner: e.target.value }))}
                    fullWidth
                    placeholder="Jane Smith"
                    helperText="Business stakeholder for this API"
                  />
                  <TextField
                    label="Business Email"
                    value={formData.businessOwnerEmail || ''}
                    onChange={(e) => setFormData((prev: any) => ({ ...prev, businessOwnerEmail: e.target.value }))}
                    fullWidth
                    type="email"
                    placeholder="jane.smith@company.com"
                    helperText="Contact for business-related inquiries"
                  />
                </Stack>
              </Stack>
            </Box>

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
                      <Box key={policy.id} sx={{ p: 2.5, border: '2px solid', borderColor: formData.subscriptionPolicies?.includes(policy.name) ? 'primary.main' : 'grey.200', borderRadius: 2, cursor: 'pointer' }} onClick={() => {
                        const isSelected = formData.subscriptionPolicies?.includes(policy.name) || false;
                        handleCheckboxChange('subscriptionPolicies', policy.name, !isSelected);
                      }}>
                        <Box display="flex" alignItems="flex-start" gap={1.5}>
                          <MUICheckbox checked={formData.subscriptionPolicies?.includes(policy.name) || false} onChange={(e) => { e.stopPropagation(); handleCheckboxChange('subscriptionPolicies', policy.name, e.target.checked); }} size="small" color="primary" />
                          <Box flex={1}>
                            <Typography variant="subtitle2" fontWeight={600} color="text.primary" mb={0.5}>{policy.name}</Typography>
                            <Typography variant="body2" color="text.secondary">{policy.description}</Typography>
                          </Box>
                        </Box>
                      </Box>
                    ))}
                  </Box>
                </Box>
              </Stack>
            </Box>

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
                    onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleAddTag(); } }}
                    size="small"
                    placeholder="e.g., mobile, finance, v2"
                    sx={{ minWidth: 200 }}
                  />
                  <Button onClick={handleAddTag} disabled={!newTag.trim()} variant="outlined" size="small">Add</Button>
                </Box>

                <Box>
                  {formData.tags && formData.tags.length > 0 ? (
                    <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1.5, p: 3, backgroundColor: 'primary.50', borderRadius: 2, border: '1px solid', borderColor: 'primary.200', minHeight: 60 }}>
                      {formData.tags?.map((tag: string) => (
                        <Chip key={tag} label={tag} onDelete={() => handleRemoveTag(tag)} size="medium" variant="filled" color="primary" />
                      ))}
                    </Box>
                  ) : (
                    <Box sx={{ p: 3, backgroundColor: 'grey.50', borderRadius: 2, border: '1px dashed', borderColor: 'grey.300', textAlign: 'center', minHeight: 60 }}>
                      <Typography variant="body2" color="text.secondary">No tags added yet. Add tags to help categorize your API.</Typography>
                    </Box>
                  )}
                </Box>
              </Box>
            </Box>

            <Box>
              <Typography variant="subtitle1" fontWeight={600} mb={1.5} color="text.primary">Documentation</Typography>
              <Stack spacing={1.5}>
                {DUMMY_DOCUMENTS.map(doc => (
                  <Box key={doc.id} sx={{ p: 2, border: '1px solid', borderColor: formData.selectedDocumentIds?.includes(doc.id) ? 'primary.main' : 'grey.200', borderRadius: 1.5, cursor: 'pointer' }} onClick={() => {
                    const isSelected = formData.selectedDocumentIds?.includes(doc.id) || false;
                    handleCheckboxChange('selectedDocumentIds', doc.id, !isSelected);
                  }}>
                    <Box display="flex" alignItems="center" gap={2}>
                      <MUICheckbox checked={formData.selectedDocumentIds?.includes(doc.id) || false} onChange={(e) => { e.stopPropagation(); handleCheckboxChange('selectedDocumentIds', doc.id, e.target.checked); }} color="primary" />
                      <Box sx={{ width: 32, height: 32, borderRadius: 1, backgroundColor: 'grey.100', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>📄</Box>
                      <Box flex={1}>
                        <Typography variant="body1" fontWeight={500} color="text.primary">{doc.name}</Typography>
                        <Typography variant="body2" color="text.secondary">{doc.size}</Typography>
                      </Box>
                    </Box>
                  </Box>
                ))}
              </Stack>
            </Box>
          </Stack>
        </Box>
      )}
    </Stack>
  );
};

export default ApiPublishForm;
