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

import { Box, CodeBlock, Paper } from '@wso2/oxygen-ui';

/**
 * Shows code, using Oxygen's `CodeBlock` for the syntax colouring.
 *
 * `CodeBlock` hands the text to Prism, which HTML-encodes whatever it
 * highlights, so a response body containing markup stays text. Two things are
 * done around it:
 *
 * - It is remounted whenever the text or language changes (`key`). Prism
 *   rewrites the element's contents in place, so letting React update that
 *   text node afterwards would fight Prism for it.
 * - Past a size where highlighting would stall the browser, the text is shown
 *   plain rather than handed to Prism at all.
 */

/** Past this, colouring costs more than it helps; the text is shown plain. */
const HIGHLIGHT_LIMIT_CHARS = 256 * 1024;

export type CodeLanguage = 'json' | 'bash' | 'markup';

/** The language to colour a payload with, from its media type. */
export const languageForMediaType = (mediaType: string | undefined): CodeLanguage =>
  /json/i.test(mediaType ?? '') ? 'json' : 'markup';

export type CodeViewProps = {
  text: string;
  language: CodeLanguage;
  maxHeight?: number;
  /** Accessible name for the scrollable region. */
  label: string;
};

export function CodeView({ text, language, maxHeight = 460, label }: CodeViewProps) {
  return (
    <Box
      aria-label={label}
      // A scrollable region must be reachable by keyboard.
      role="region"
      sx={{ maxHeight, overflow: 'auto' }}
      tabIndex={0}
    >
      {text.length > HIGHLIGHT_LIMIT_CHARS ? (
        <Paper component="pre" sx={{ m: 0, overflow: 'auto', p: 2 }} variant="outlined">
          {text}
        </Paper>
      ) : (
        <CodeBlock code={text} key={`${language}:${text}`} language={language} />
      )}
    </Box>
  );
}
