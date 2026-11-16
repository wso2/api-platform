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

import { Box, Footer, PageContent } from '@wso2/oxygen-ui';
import type { PageContentProps } from '@wso2/oxygen-ui';
import { createContext, useContext } from 'react';
import { FormattedMessage } from 'react-intl';

import { runtimeConfig } from '../config/runtime';
import { APP_FOOTER_ID } from '../pages/appShell/appLayoutConstants';

/** Every footer link leaves the console, so each opens in a new tab, cut off from this page. */
const NEW_TAB = { target: '_blank', rel: 'noopener noreferrer' } as const;

/**
 * True below an `AppPageContent`, which has already put the footer under its
 * content. A page container nested inside one (a built-in page an extension
 * renders, say) must not add a second.
 */
const FooterProvidedContext = createContext(false);

/**
 * The console's footer, in Oxygen's `plain` variant (no bar, an inset divider).
 *
 * The id is an anchor for measuring the footer, so a page that fills the scroll
 * area (`useFillScrollArea`) can leave room for it.
 */
function PageFooter() {
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

export type AppPageContentProps = PageContentProps;

/**
 * The console's one page container: Oxygen's `PageContent` with the console
 * footer passed through its `footer` prop, so a short page leaves the footer at
 * the bottom and a long one pushes it down after the content.
 *
 * Every page uses this, directly or through `AppPage`, so the footer is defined
 * once. Pass `footer` to replace it (`footer={null}` for none). Wrapped around
 * an extension's own page it gives that page the footer too (`noPadding`, as the
 * page brings its own); only the outermost one renders a footer.
 */
export function AppPageContent({ children, footer, ...pageContentProps }: AppPageContentProps) {
  const footerProvided = useContext(FooterProvidedContext);
  const resolvedFooter = footer !== undefined ? footer : footerProvided ? null : <PageFooter />;
  return (
    <PageContent footer={resolvedFooter} {...pageContentProps}>
      <FooterProvidedContext.Provider value={footerProvided || resolvedFooter !== null}>
        {children}
      </FooterProvidedContext.Provider>
    </PageContent>
  );
}
