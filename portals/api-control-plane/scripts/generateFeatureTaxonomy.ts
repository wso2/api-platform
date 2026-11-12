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

/**
 * Generates src/featureUsage/taxonomy.generated.ts from the API Platform
 * feature taxonomy, src/featureUsage/api-platform-taxonomy.yaml.
 *
 *   npm run taxonomy:codegen
 *
 * The output holds the whole taxonomy (every feature, for every surface) and,
 * separately, the features the console emits: those whose `portal` dimension
 * names "api-control-plane". The console's `portal` is not part of a console
 * feature's dimensions, because the console adds it to every event itself.
 *
 * Committed, not built: the cloud console is built from this directory alone,
 * and a generated file shows each taxonomy change as a reviewable diff.
 */

import { readFileSync, writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import yaml from 'js-yaml';
import * as prettier from 'prettier';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const TAXONOMY_PATH = path.resolve(HERE, '../src/featureUsage/api-platform-taxonomy.yaml');
const OUT_PATH = path.resolve(HERE, '../src/featureUsage/taxonomy.generated.ts');

/** The dimension naming the UI a web feature comes from. */
const PORTAL_DIMENSION = 'portal';
/** This console's `portal` value. */
const CONSOLE_PORTAL = 'api-control-plane';
/** Every `portal` value the taxonomy may use. */
const PORTALS = ['api-control-plane', 'api-portal'];

type Dimension = {
  name: string;
  type: string;
  values?: string[];
  required?: boolean;
  description?: string;
};

type Feature = {
  code: string;
  name: string;
  category: string;
  tier: string;
  tracking: {
    event_category: string;
    platforms?: string[];
    dimensions?: Dimension[] | null;
  };
  sub_features?: Feature[];
};

type Taxonomy = { product: { id: string }; features: Feature[] };

const problems: string[] = [];

const taxonomy = yaml.load(readFileSync(TAXONOMY_PATH, 'utf8')) as Taxonomy;

/** Every feature, sub-features included, in file order. */
const allFeatures = (features: Feature[]): Feature[] =>
  features.flatMap((feature) => [feature, ...allFeatures(feature.sub_features ?? [])]);

const consoleFeatures: Record<string, unknown> = {};

for (const feature of allFeatures(taxonomy.features)) {
  const dimensions = feature.tracking.dimensions ?? [];
  const portal = dimensions.find((dimension) => dimension.name === PORTAL_DIMENSION);
  const platforms = feature.tracking.platforms ?? [];

  if (portal) {
    if (!platforms.includes('web')) {
      problems.push(`${feature.code}: has a portal dimension but is not a web feature`);
    }
    const unknown = (portal.values ?? []).filter((value) => !PORTALS.includes(value));
    if (portal.type !== 'enum' || unknown.length > 0 || (portal.values ?? []).length === 0) {
      problems.push(`${feature.code}: portal must be an enum of ${PORTALS.join(', ')}`);
    }
  }
  if (!portal?.values?.includes(CONSOLE_PORTAL)) continue;

  const consoleDimensions: Record<string, { required: boolean; values: string[] }> = {};
  for (const dimension of dimensions) {
    if (dimension.name === PORTAL_DIMENSION) continue;
    // The console sends enums only, so a call site can't forward user input.
    if (dimension.type !== 'enum' || !dimension.values?.length) {
      problems.push(`${feature.code}: console dimension "${dimension.name}" is not an enum`);
      continue;
    }
    consoleDimensions[dimension.name] = {
      required: dimension.required === true,
      values: dimension.values,
    };
  }
  consoleFeatures[feature.code] = {
    name: feature.name,
    category: feature.category,
    tier: feature.tier,
    eventCategory: feature.tracking.event_category,
    dimensions: consoleDimensions,
  };
}

if (problems.length > 0) {
  console.error(`${path.relative(process.cwd(), TAXONOMY_PATH)}:`);
  for (const problem of problems) console.error(`  - ${problem}`);
  process.exit(1);
}

const LICENSE = readFileSync(fileURLToPath(import.meta.url), 'utf8').split('*/')[0] + '*/';

const source = `${LICENSE}

/**
 * GENERATED FILE — do not edit.
 *
 * Produced by \`scripts/generateFeatureTaxonomy.ts\` (\`npm run taxonomy:codegen\`)
 * from src/featureUsage/api-platform-taxonomy.yaml. Edit the YAML, not this file.
 */

/** A dimension, as the WSO2 feature-taxonomy schema (v1) defines it. */
export type TaxonomyDimension = {
  name: string;
  type: 'string' | 'integer' | 'float' | 'boolean' | 'enum';
  values?: readonly string[];
  required?: boolean;
  description?: string;
};

/** A feature or sub-feature, as the WSO2 feature-taxonomy schema (v1) defines it. */
export type TaxonomyFeature = {
  code: string;
  name: string;
  description?: string;
  category: string;
  tier: 'core' | 'premium' | 'enterprise';
  status: 'stable' | 'beta' | 'experimental' | 'deprecated';
  availability: {
    introduced_in: string;
    deprecated_in?: string | null;
    version_tier_override?: string | null;
  };
  tracking: {
    event_name: string;
    event_category: 'discovery' | 'engagement' | 'adoption' | 'abandonment' | 'lifecycle';
    platforms?: readonly ('web' | 'mobile-ios' | 'mobile-android' | 'desktop' | 'api')[];
    dimensions?: readonly TaxonomyDimension[];
    aggregations?: readonly { type: string; label: string; dimension?: string | null }[];
  };
  sub_features?: readonly TaxonomyFeature[];
};

/** A category, as the WSO2 feature-taxonomy schema (v1) defines it. */
export type TaxonomyCategory = {
  id: string;
  name: string;
  description?: string;
  parent_category_id?: string | null;
  sub_categories?: readonly TaxonomyCategory[];
};

/** A product taxonomy, as the WSO2 feature-taxonomy schema (v1) defines it. */
export type FeatureTaxonomy = {
  schema: { version: string; id: string };
  product: {
    id: string;
    name: string;
    version: string;
    description?: string;
    team_owner?: string;
    category?: string;
    is_active?: boolean;
  };
  categories: readonly TaxonomyCategory[];
  features: readonly TaxonomyFeature[];
};

/** The whole API Platform feature taxonomy: every feature, for every surface. */
export const API_PLATFORM_TAXONOMY: FeatureTaxonomy = ${JSON.stringify(taxonomy)};

/** This console's \`portal\` value, sent with every event it emits. */
export const CONSOLE_PORTAL = ${JSON.stringify(CONSOLE_PORTAL)};

/**
 * The features the console emits: every taxonomy feature whose \`portal\`
 * names "${CONSOLE_PORTAL}", keyed by code, with the fields the console sends.
 * \`portal\` itself is left out of the dimensions: the console adds it.
 */
export const CONSOLE_FEATURES = ${JSON.stringify(consoleFeatures)} as const;
`;

const prettierConfig = await prettier.resolveConfig(OUT_PATH);
const formatted = await prettier.format(source, {
  ...prettierConfig,
  filepath: OUT_PATH,
});

writeFileSync(OUT_PATH, formatted, 'utf8');
console.log(
  `Wrote ${path.relative(process.cwd(), OUT_PATH)}: ${allFeatures(taxonomy.features).length} features, ` +
    `${Object.keys(consoleFeatures).length} emitted by the console.`,
);
