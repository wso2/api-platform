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
import { Box, Stack, Typography } from '@wso2/oxygen-ui';
import NoData from '../../assets/images/NoData.svg';

interface NoSearchResultsProps {
  /** e.g. "No providers match your search." */
  message: React.ReactNode;
}

/**
 * Shared "nothing matched this search" illustration + message, for use below
 * a list page's search box once a query returns zero results. Reuses the
 * same NoData.svg illustration already used for other empty-collection states
 * (ServiceProviderResourcesTab, LLMProxyResourcesTab, etc.) so empty states
 * look consistent across the app.
 */
export default function NoSearchResults({ message }: NoSearchResultsProps) {
  return (
    <Stack
      spacing={1.5}
      alignItems="center"
      justifyContent="center"
      sx={{ textAlign: 'center', py: 6 }}
    >
      <Box
        component="img"
        src={NoData}
        alt="No results"
        sx={{ width: 80, maxWidth: '80%' }}
      />
      <Typography variant="body2" color="text.secondary">
        {message}
      </Typography>
    </Stack>
  );
}
