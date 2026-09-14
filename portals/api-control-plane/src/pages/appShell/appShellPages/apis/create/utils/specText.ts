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

/**
 * Turning a definition into text a person can edit, and back again.
 *
 * The document the wizard carries is a parsed object, not the bytes it arrived
 * as, so text is something this module *produces* rather than something it
 * preserves: an uploaded YAML file's comments, key order and anchors are
 * already gone by the time anything here runs. That makes the format a display
 * choice; the same object printed as JSON or as YAML; rather than a property
 * of the document.
 */

/** How the source pane prints a definition, and reads one back. */
export type SpecFormat = 'json' | 'yaml';

/** A parsed definition, as loose as it arrives — nothing here inspects it. */
export type SpecDocument = Record<string, unknown>;

export type SpecParseResult =
  | { spec: SpecDocument; status: 'parsed' }
  /** Not readable as the chosen format. `reason` is the parser's own message. */
  | { reason: string; status: 'malformed' }
  /** Read fine, but the top level is a list, a string, a number, or null. */
  | { status: 'notAnObject' };

/** Indentation used by both formats, so switching between them doesn't reflow. */
const INDENT = 2;

/**
 * Prints the document.
 *
 * `lineWidth: -1` keeps every scalar on one line: YAML would otherwise fold a
 * long `description` across several, which is correct but reads as damage to
 * someone who came here to change one field. `noRefs` spells a repeated object
 * out in full rather than emitting an anchor and an alias.
 *
 * Never throws — a definition that `dump` refuses (an `undefined` somewhere a
 * parse can't produce, but a future caller might) falls back to JSON rather
 * than taking the pane down with it.
 */
export const serializeSpec = (spec: SpecDocument, format: SpecFormat): string => {
  if (format === 'yaml') {
    try {
      return yaml.dump(spec, { indent: INDENT, lineWidth: -1, noRefs: true });
    } catch {
      // Fall through to JSON, which is a subset of YAML and so still parses
      // back in either format.
    }
  }
  return JSON.stringify(spec, null, INDENT);
};

/**
 * Reads edited text back into a document.
 *
 * Parsed with the format the editor is actually in rather than always through
 * js-yaml: JSON is a subset of YAML, so a YAML parse would quietly accept text
 * the user believes is JSON and report a stray comma at the wrong place. The
 * parser's own message carries the line and column, which is the part that
 * makes a syntax error fixable, so it rides out as data.
 */
export const parseSpecText = (text: string, format: SpecFormat): SpecParseResult => {
  let parsed: unknown;
  try {
    // `load` uses the default schema, which builds plain data only, never
    // arbitrary JS types.
    parsed = format === 'yaml' ? yaml.load(text) : JSON.parse(text);
  } catch (error) {
    return {
      reason: error instanceof Error ? error.message : String(error),
      status: 'malformed',
    };
  }

  if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
    return { status: 'notAnObject' };
  }
  return { spec: parsed as SpecDocument, status: 'parsed' };
};
