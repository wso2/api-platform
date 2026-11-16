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

import { Box, Footer } from '@wso2/oxygen-ui';
import { FormattedMessage } from 'react-intl';

import { runtimeConfig } from '../config/runtime';
import { APP_FOOTER_ID } from '../pages/appShell/appLayoutConstants';

/** Every footer link leaves the console, so each opens in a new tab, cut off from this page. */
const NEW_TAB = { target: '_blank', rel: 'noopener noreferrer' } as const;

/**
 * The console's footer, handed to `PageContent` through its `footer` prop (see
 * `AppPage`) so it sits at the bottom of a short page and follows the content of
 * a long one, instead of being pinned under the shell. The `plain` variant: no
 * bar behind it, just an inset divider, so it reads as part of the page.
 *
 * The id is an anchor for measuring the footer, so a page that fills the scroll
 * area (`useFillScrollArea`) can leave room for it.
 */
export function AppFooter() {
  return (
    <Box id={APP_FOOTER_ID}>
      <Footer variant="plain">
        <Footer.Copyright>© {new Date().getFullYear()} WSO2 LLC.</Footer.Copyright>
        {/* Show the environment label only for pinned on-prem builds. */}
        {runtimeConfig.deploymentMode === 'onprem' && (
          <Footer.Version>{runtimeConfig.environmentName}</Footer.Version>
        )}
        <Footer.Link href={runtimeConfig.documentationLink} {...NEW_TAB}>
          <FormattedMessage
            id="appLayout.footer.documentation"
            defaultMessage="Documentation"
            description="Footer link to the product documentation"
          />
        </Footer.Link>
        <Footer.Link href={runtimeConfig.supportLink} {...NEW_TAB}>
          <FormattedMessage
            id="appLayout.footer.support"
            defaultMessage="Support"
            description="Footer link to the community support channel"
          />
        </Footer.Link>
        <Footer.Link href={runtimeConfig.termsOfUseLink} {...NEW_TAB}>
          <FormattedMessage
            id="appLayout.footer.termsOfUse"
            defaultMessage="Terms of Use"
            description="Footer link to the Terms of Use page"
          />
        </Footer.Link>
        <Footer.Link href={runtimeConfig.privacyPolicyLink} {...NEW_TAB}>
          <FormattedMessage
            id="appLayout.footer.privacyPolicy"
            defaultMessage="Privacy Policy"
            description="Footer link to the Privacy Policy page"
          />
        </Footer.Link>
      </Footer>
    </Box>
  );
}
