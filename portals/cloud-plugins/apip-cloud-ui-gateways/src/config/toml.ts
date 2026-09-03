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
 * The one guard `gateway.config_toml` gets, and it is only a warning.
 *
 * The chart appends this text, verbatim and last, to a config.toml it has
 * already written, and TOML forbids declaring a table twice — so a block
 * repeating a section the structured values already emitted is a parse error at
 * gateway startup rather than an override. The bootstrap seed emits two such
 * sections, so those two in particular cannot be re-declared.
 *
 * NOTHING ON THE SERVER CHECKS THIS. It is the known gap the field shipped
 * with, and a client-side scan does not close it: a request that skips this UI
 * still gets a 200 and a gateway that will not start. What this buys is that
 * someone typing into the form is told before they find out from a dead
 * gateway.
 */

/** The sections the bootstrap seed already emits. */
export const SEEDED_SECTIONS = [
  'policy_configurations.ratelimit_v1',
  'policy_configurations.llm_cost_ratelimit_v1',
];

/**
 * Table headers at the start of a line, which is the only position TOML
 * declares one in. Deliberately not a TOML parser: a partially-typed document
 * is the normal state of a text area, and a parser would spend most of its life
 * reporting syntax errors about text the user has not finished writing.
 */
// A header may be followed by a comment: TOML treats `#` as a comment to the
// end of the line, so `[policy_configurations.ratelimit_v1] # note` declares
// that table just as surely as the bare form. Missing it meant the redeclared
// -section warning stayed silent for the exact text that kills a gateway.
const SECTION_HEADER = /^[ \t]*\[([^\]]+)\][ \t]*(?:#[^\n]*)?$/gm;

/** Every table header the text declares, in order of appearance. */
export function tomlSections(text: string): string[] {
  const found: string[] = [];
  SECTION_HEADER.lastIndex = 0;
  for (
    let match = SECTION_HEADER.exec(text);
    match;
    match = SECTION_HEADER.exec(text)
  ) {
    found.push(match[1].trim());
  }
  return found;
}

/** The seeded sections this text re-declares. A warning, never a block. */
export function redeclaredSections(text: string): string[] {
  return [
    ...new Set(
      tomlSections(text).filter((section) => SEEDED_SECTIONS.includes(section))
    ),
  ];
}
