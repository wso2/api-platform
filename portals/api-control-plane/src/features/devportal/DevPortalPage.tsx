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

import {
  Box,
  Button,
  InputAdornment,
  PageContent,
  PageTitle,
  TextField,
} from '@wso2/oxygen-ui';
import { Plus, Search } from '@wso2/oxygen-ui-icons-react';
import { useState } from 'react';

import { EmptyState } from '../../components/StateViews';

export function DevPortalPage() {
  const [search, setSearch] = useState('');

  const provision = () => {
    // Devportal provisioning is not implemented yet.
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <PageTitle.Header>Dev Portal</PageTitle.Header>
        <PageTitle.SubHeader>
          Provision and manage the developer portal for your organization.
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            onClick={provision}
            startIcon={<Plus />}
            sx={{ borderRadius: 5 }}
            variant="contained"
          >
            Provision Devportal
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <Box sx={{ mb: 3 }}>
        <TextField
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search Dev Portal"
          size="small"
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={18} />
                </InputAdornment>
              ),
            },
          }}
          sx={{ maxWidth: 420, minWidth: 240, width: '100%' }}
          value={search}
        />
      </Box>

      <EmptyState
        actionLabel="Provision Devportal"
        description="Provision a developer portal to publish APIs for external developers."
        onAction={provision}
        title="No Dev Portal provisioned yet"
      />
    </PageContent>
  );
}
