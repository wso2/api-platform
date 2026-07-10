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

import React from 'react';
import { Box, PageContent, Stack, Typography } from '@wso2/oxygen-ui';
import ComingSoon from '../../../../assets/images/ComingSoon.svg';
import { FormattedMessage } from 'react-intl';

export default function Registries(): JSX.Element {
  return (
    <PageContent fullWidth>
      <Box
        sx={{
          minHeight: '60vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          textAlign: 'center',
          px: 2,
        }}
      >
        <Stack spacing={2} alignItems="center">
          <Box
            component="img"
            src={ComingSoon}
            alt="Coming soon"
            sx={{ width: 300, maxWidth: '100%' }}
          />
          <Box
            display="flex"
            flexDirection="column"
            alignItems="center"
            gap={1}
            mt={2}
            maxWidth={300}
          >
            <Typography variant="h3" sx={{ fontWeight: 700 }}>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.registries.Main.coming.soon"
                defaultMessage={'Coming Soon'}
              />
            </Typography>
            <Typography variant="body2" color="text.secondary">
              <FormattedMessage
                id="aiWorkspace.pages.appShell.appShellPages.registries.Main.a.centralized.place.to.manage.registries.will.be.available.soon"
                defaultMessage={
                  'A centralized place to manage registries will be available soon.'
                }
              />
            </Typography>
          </Box>
        </Stack>
      </Box>
    </PageContent>
  );
}
