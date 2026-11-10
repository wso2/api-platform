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

import { ColorSchemeImage, Stack, Typography } from '@wso2/oxygen-ui';
import { FormattedMessage } from 'react-intl';

import { useBrandLogo } from './BrandLogoProvider';

/**
 * The product mark with its name set in Oxygen typography: "API" bold,
 * "Management" regular. Shared by the app header and the login page so both
 * paint the same lockup, on-prem and cloud alike.
 */
export function BrandLockup({ height }: { height: number }) {
  const brandLogo = useBrandLogo();

  return (
    <Stack alignItems="center" direction="row" spacing={1.5}>
      <ColorSchemeImage alt="" height={height} src={brandLogo} width="auto" />
      <Typography
        component="span"
        sx={{ fontSize: height * 0.75, lineHeight: 1, whiteSpace: 'nowrap' }}
        variant="h6"
      >
        <FormattedMessage
          defaultMessage="<bold>API</bold> Management"
          description="Product name in the brand lockup. API is bold, Management regular — keep the bold tag around the equivalent of API."
          id="branding.BrandLockup.name"
          values={{ bold: (chunks) => <b>{chunks}</b> }}
        />
      </Typography>
    </Stack>
  );
}
