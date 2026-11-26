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

import { ColorSchemeImage } from '@wso2/oxygen-ui';
import { useIntl } from 'react-intl';

import { useBrandLogo } from './BrandLogoProvider';

/**
 * The product logo. The artwork is the full wordmark (mark plus the name), so
 * nothing is drawn beside it; the name only reaches assistive technology, as
 * the image's alt text.
 */
export function BrandLockup({
  height,
}: {
  /** Logo height: pixels as a number, or any CSS length such as `1.75rem`. */
  height: number | string;
}) {
  const brandLogo = useBrandLogo();
  const intl = useIntl();

  return (
    <ColorSchemeImage
      alt={intl.formatMessage({
        defaultMessage: 'Choreo API Management',
        description:
          'Alt text of the product logo, which already contains the name. Choreo is a product name — leave it untranslated.',
        id: 'branding.BrandLockup.name',
      })}
      height={height}
      src={brandLogo}
      sx={{ display: 'block', maxWidth: '100%' }}
      width="auto"
    />
  );
}
