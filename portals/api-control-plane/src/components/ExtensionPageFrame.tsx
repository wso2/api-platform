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

import { Box } from '@wso2/oxygen-ui';
import { createContext, useContext, type ReactNode } from 'react';

import { AppFooter } from './AppFooter';

/**
 * True below an `ExtensionPageFrame`, which has already put the footer under the
 * page. A built-in page rendered inside one (the Deploy page's API picker, shown
 * by an extension at organization or project level) must not add its own, or the
 * page ends up with two.
 */
const FooterProvidedContext = createContext(false);

/** Whether an enclosing frame already renders the footer. */
export const useFooterProvided = () => useContext(FooterProvidedContext);

/**
 * Gives a page an extension (cloud plugin) renders the console footer.
 *
 * Built-in pages get the footer from `AppPage`, through `PageContent`'s `footer`
 * prop. An extension's page is its own `PageContent`, in a package that cannot
 * import this portal's components, so the host adds the footer around it: a short
 * page leaves the footer at the bottom of the scroll area, a long one pushes it
 * down after the content — the same behaviour `PageContent` gives its own footer.
 *
 * Only one footer is ever shown: inside another frame this renders its children
 * alone, and `AppPage` leaves its footer off (see `useFooterProvided`).
 */
export function ExtensionPageFrame({ children }: { children: ReactNode }) {
  const provided = useFooterProvided();
  if (provided) return <>{children}</>;
  return (
    <FooterProvidedContext.Provider value>
      <Box sx={{ display: 'flex', flexDirection: 'column', minHeight: '100%', width: '100%' }}>
        <Box sx={{ flex: '1 0 auto', minWidth: 0 }}>{children}</Box>
        <AppFooter />
      </Box>
    </FooterProvidedContext.Provider>
  );
}
