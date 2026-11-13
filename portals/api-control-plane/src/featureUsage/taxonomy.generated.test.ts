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

import yaml from 'js-yaml';
import { describe, expect, it } from 'vitest';

import taxonomyYaml from './api-platform-taxonomy.yaml?raw';
import {
  API_PLATFORM_PRODUCT,
  API_PLATFORM_TAXONOMY,
  CONSOLE_FEATURES,
  CONSOLE_PORTAL,
  type FeatureTaxonomy,
  type TaxonomyFeature,
} from './taxonomy.generated';

/*
 * taxonomy.generated.ts is generated from api-platform-taxonomy.yaml
 * (`npm run taxonomy:codegen`). These tests fail when the YAML changes without
 * regenerating, and check the rules the console relies on: which features are
 * the console's, and how the `portal` dimension is used.
 */

const fromYaml = yaml.load(taxonomyYaml) as FeatureTaxonomy;

const allFeatures = (features: readonly TaxonomyFeature[]): TaxonomyFeature[] =>
  features.flatMap((feature) => [feature, ...allFeatures(feature.sub_features ?? [])]);

const features = allFeatures(fromYaml.features);
const portalOf = (feature: TaxonomyFeature) =>
  feature.tracking.dimensions?.find((dimension) => dimension.name === 'portal');

describe('the generated taxonomy', () => {
  it('is the whole of api-platform-taxonomy.yaml', () => {
    expect(API_PLATFORM_TAXONOMY).toEqual(fromYaml);
  });

  it('lists as the console’s exactly the features whose portal is api-control-plane', () => {
    const expected = features.filter((feature) =>
      portalOf(feature)?.values?.includes(CONSOLE_PORTAL),
    );

    expect(Object.keys(CONSOLE_FEATURES)).toEqual(expected.map((feature) => feature.code));
    for (const feature of expected) {
      expect(CONSOLE_FEATURES[feature.code as keyof typeof CONSOLE_FEATURES]).toEqual({
        name: feature.name,
        category: feature.category,
        tier: feature.tier,
        eventCategory: feature.tracking.event_category,
        dimensions: Object.fromEntries(
          (feature.tracking.dimensions ?? [])
            .filter((dimension) => dimension.name !== 'portal')
            .map((dimension) => [
              dimension.name,
              { required: dimension.required === true, values: dimension.values },
            ]),
        ),
      });
    }
  });
});

describe('the portal dimension', () => {
  const withPortal = features.filter((feature) => portalOf(feature));

  it('is declared only by web features', () => {
    for (const feature of withPortal) {
      expect(feature.tracking.platforms, feature.code).toContain('web');
    }
  });

  it('names only the API Platform portals', () => {
    for (const feature of withPortal) {
      const portal = portalOf(feature);
      expect(portal?.type, feature.code).toBe('enum');
      expect(portal?.values?.length, feature.code).toBeGreaterThan(0);
      for (const value of portal?.values ?? []) {
        expect(['api-control-plane', 'api-portal'], feature.code).toContain(value);
      }
    }
  });

  it('is required for web-only features, and optional for those also reported from the API', () => {
    for (const feature of withPortal) {
      const webOnly =
        feature.tracking.platforms?.length === 1 && feature.tracking.platforms[0] === 'web';
      expect(portalOf(feature)?.required === true, feature.code).toBe(webOnly);
    }
  });

  it('is never declared by an API-only feature', () => {
    for (const feature of features.filter((f) => !f.tracking.platforms?.includes('web'))) {
      expect(portalOf(feature), feature.code).toBeUndefined();
    }
  });
});

describe('the product', () => {
  it('is the taxonomy product: its id prefixes every code, its name is sent as platform', () => {
    expect(API_PLATFORM_PRODUCT).toEqual({
      id: fromYaml.product.id,
      name: fromYaml.product.name,
    });
  });
});

describe('console dimensions', () => {
  // The fields every action carries (featureMetadata); a dimension may not shadow one.
  const RESERVED = [
    'growth_action',
    'platform',
    'source',
    'environment',
    'action',
    'category',
    'tier',
    'event_category',
    'initiating_product',
  ];

  it.each(Object.entries(CONSOLE_FEATURES))('%s uses no reserved metadata key', (_code, spec) => {
    for (const name of Object.keys(spec.dimensions)) {
      expect(RESERVED).not.toContain(name);
    }
  });
});
