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
 * Re-indents JSON text without changing any value in it.
 *
 * `JSON.stringify(JSON.parse(text))` reads every number as a double, so an
 * integer above 2^53 (an ID, a ledger amount) comes back altered. This walks
 * the characters instead and only rewrites the whitespace between tokens;
 * strings and number tokens are copied exactly as written.
 *
 * Throws `SyntaxError` when the text is not valid JSON, so callers can keep
 * the original on failure.
 */
export const prettyPrintJson = (text: string, indent = '  '): string => {
  // Validation only: the result is discarded, so nothing is lost to rounding.
  JSON.parse(text);

  let out = '';
  let depth = 0;
  let inString = false;
  let escaped = false;
  // The previous significant token opened a container, so an immediate close stays `{}`.
  let justOpened = false;

  for (const char of text) {
    if (inString) {
      out += char;
      if (escaped) escaped = false;
      else if (char === '\\') escaped = true;
      else if (char === '"') inString = false;
      continue;
    }
    // First token inside a container goes on its own line (unless the container is empty).
    if (justOpened && !/\s/.test(char) && char !== '}' && char !== ']') {
      out += `\n${indent.repeat(depth)}`;
      justOpened = false;
    }
    switch (char) {
      case '"':
        inString = true;
        justOpened = false;
        out += char;
        break;
      case '{':
      case '[':
        out += char;
        depth += 1;
        justOpened = true;
        break;
      case '}':
      case ']':
        depth -= 1;
        if (!justOpened) out += `\n${indent.repeat(depth)}`;
        out += char;
        justOpened = false;
        break;
      case ',':
        out += `,\n${indent.repeat(depth)}`;
        justOpened = false;
        break;
      case ':':
        out += ': ';
        break;
      case ' ':
      case '\t':
      case '\n':
      case '\r':
        break;
      default:
        out += char;
    }
  }
  return out;
};
