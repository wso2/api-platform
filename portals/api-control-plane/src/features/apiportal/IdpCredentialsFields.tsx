/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useState } from 'react';
import {
  Box,
  FormControl,
  FormLabel,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Eye, EyeOff } from '@wso2/oxygen-ui-icons-react';

import { isValidUrl } from '../apis/develop/developEdit';

type IdpCredentialsFieldsProps = {
  stsTokenUrl: string;
  onStsTokenUrlChange: (value: string) => void;
  clientId: string;
  onClientIdChange: (value: string) => void;
  clientSecret: string;
  onClientSecretChange: (value: string) => void;
};

/**
 * Grouped STS token URL / client ID / client secret inputs shown once
 * `authType` is `oauth2` — shared by the create and detail/edit API Portal
 * pages so the group never drifts between the two.
 */
export function IdpCredentialsFields({
  stsTokenUrl,
  onStsTokenUrlChange,
  clientId,
  onClientIdChange,
  clientSecret,
  onClientSecretChange,
}: IdpCredentialsFieldsProps) {
  const [secretVisible, setSecretVisible] = useState(false);
  const stsTokenUrlInvalid = stsTokenUrl.trim() !== '' && !isValidUrl(stsTokenUrl);

  return (
    <Box
      sx={{
        border: '1px solid',
        borderColor: 'divider',
        borderRadius: 2,
        p: 2.25,
      }}
    >
      <Typography
        color="text.secondary"
        sx={{
          display: 'block',
          fontWeight: 600,
          letterSpacing: '.12em',
          mb: 1.5,
        }}
        variant="caption"
      >
        IDP CLIENT CREDENTIALS
      </Typography>
      <Stack spacing={2}>
        <FormControl fullWidth>
          <FormLabel htmlFor="idp-sts-token-url">STS token URL</FormLabel>
          <TextField
            id="idp-sts-token-url"
            error={stsTokenUrlInvalid}
            helperText={stsTokenUrlInvalid ? 'Enter a valid URL' : undefined}
            onChange={(event) => onStsTokenUrlChange(event.target.value)}
            placeholder="https://idp.example.com/oauth2/token"
            value={stsTokenUrl}
          />
        </FormControl>

        <FormControl fullWidth>
          <FormLabel htmlFor="idp-client-id">Client ID</FormLabel>
          <TextField
            id="idp-client-id"
            onChange={(event) => onClientIdChange(event.target.value)}
            value={clientId}
          />
        </FormControl>

        <FormControl fullWidth>
          <FormLabel htmlFor="idp-client-secret">Client secret</FormLabel>
          <TextField
            id="idp-client-secret"
            onChange={(event) => onClientSecretChange(event.target.value)}
            slotProps={{
              input: {
                endAdornment: (
                  <InputAdornment position="end">
                    <IconButton
                      aria-label={
                        secretVisible
                          ? 'Hide client secret'
                          : 'Show client secret'
                      }
                      onClick={() => setSecretVisible((v) => !v)}
                      size="small"
                    >
                      {secretVisible ? <EyeOff size={16} /> : <Eye size={16} />}
                    </IconButton>
                  </InputAdornment>
                ),
              },
            }}
            type={secretVisible ? 'text' : 'password'}
            value={clientSecret}
          />
        </FormControl>
      </Stack>
    </Box>
  );
}
