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

import { createContext, useContext, type ReactNode } from 'react';

import onPremLogoDark from '@/assets/icons/logos/apiplatform_white.svg';
import onPremLogoLight from '@/assets/icons/logos/apiplatform_black.svg';

/**
 * Brand logo sources per color scheme, in the shape `ColorSchemeImage`'s `src`
 * prop expects: `dark` is the mark drawn *on* a dark background (the white
 * one), `light` the mark drawn on a light background.
 */
export type BrandLogo = {
  dark: string;
  light: string;
};

/**
 * The on-prem product's own logos — what a plain build of this portal paints,
 * and the placeholder a host app replaces.
 *
 * This portal never ships cloud artwork: the cloud console owns its own logos
 * (see `portals/cloud-plugins/apip-cloud-ui`) and hands them to `App` as
 * `brandLogo`, the same way it hands over its `extensions`.
 */
export const defaultBrandLogo: BrandLogo = {
  dark: onPremLogoDark,
  light: onPremLogoLight,
};

/**
 * Defaulted rather than `undefined`-with-a-throw: a component rendered outside
 * the provider (a test mounting the header on its own, say) should still paint
 * the product's own logo rather than fail.
 */
const BrandLogoContext = createContext<BrandLogo>(defaultBrandLogo);

export function BrandLogoProvider({
  brandLogo = defaultBrandLogo,
  children,
}: {
  brandLogo?: BrandLogo;
  children: ReactNode;
}) {
  return <BrandLogoContext.Provider value={brandLogo}>{children}</BrandLogoContext.Provider>;
}

/** The logo every brand surface (app header, login page) paints. */
export function useBrandLogo(): BrandLogo {
  return useContext(BrandLogoContext);
}
