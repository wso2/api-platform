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

import { createContext, useContext, type ReactNode } from "react";

import onPremLogoDark from "../assets/images/AIWorkspaceLogo.svg";
import onPremLogoLight from "../assets/images/AIWorkspaceLogoDark.svg";

/**
 * Brand logo sources per color scheme, in the shape `ColorSchemeImage`'s `src`
 * prop expects: `dark` is the mark drawn *on* a dark background, `light` the
 * mark drawn on a light background.
 */
export type BrandLogo = {
  dark: string;
  light: string;
};

/**
 * Default on-premise logos, replaceable by the host application.
 * Cloud logos are supplied by the cloud console through `brandLogo`.
 */
export const defaultBrandLogo: BrandLogo = {
  dark: onPremLogoDark,
  light: onPremLogoLight,
};

/** Uses the product logo when rendered outside a provider. */
const BrandLogoContext = createContext<BrandLogo>(defaultBrandLogo);

export function BrandLogoProvider({
  brandLogo = defaultBrandLogo,
  children,
}: {
  brandLogo?: BrandLogo;
  children: ReactNode;
}) {
  return (
    <BrandLogoContext.Provider value={brandLogo}>
      {children}
    </BrandLogoContext.Provider>
  );
}

/** The logo every brand surface (`Logo`, and so the header and login pages) paints. */
export function useBrandLogo(): BrandLogo {
  return useContext(BrandLogoContext);
}
