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

import { useEffect, useRef, useState } from 'react';
import { Box, Tooltip } from '@wso2/oxygen-ui';

export type TruncatedTextProps = {
  text: string;
  /** Maximum visible lines. Defaults to one. */
  maxLines?: number;
  /** Maximum characters before the ellipsis, independent of the line limit. */
  maxCharacters?: number;
};

/**
 * Splits text into user-perceived characters, so a limit never cuts inside an
 * accent, a flag or a joined emoji. Falls back to code points where
 * `Intl.Segmenter` is unavailable.
 */
const graphemes = (text: string): string[] =>
  typeof Intl !== 'undefined' && 'Segmenter' in Intl
    ? Array.from(new Intl.Segmenter(undefined, { granularity: 'grapheme' }).segment(text), (part) => part.segment)
    : Array.from(text);

/** Inherits typography from its parent and reveals clipped text on hover or focus. */
export function TruncatedText({ text, maxLines = 1, maxCharacters }: TruncatedTextProps) {
  const ref = useRef<HTMLSpanElement>(null);
  const [overflows, setOverflows] = useState(false);
  const characters = graphemes(text);
  const limit = maxCharacters === undefined ? undefined : Math.max(0, Math.floor(maxCharacters));
  const shortened = limit !== undefined && characters.length > limit;
  const visibleText = shortened ? `${characters.slice(0, limit).join('')}…` : text;
  const lines = Math.max(1, Math.floor(maxLines));

  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const measure = () => {
      setOverflows(
        element.scrollHeight > element.clientHeight || element.scrollWidth > element.clientWidth,
      );
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [visibleText, lines]);

  const truncated = shortened || overflows;

  return (
    <Tooltip title={truncated ? text : ''} describeChild>
      <Box
        component="span"
        ref={ref}
        tabIndex={truncated ? 0 : undefined}
        sx={{
          display: '-webkit-box',
          WebkitBoxOrient: 'vertical',
          WebkitLineClamp: lines,
          overflow: 'hidden',
          overflowWrap: 'anywhere',
          whiteSpace: 'normal',
        }}
      >
        {visibleText}
      </Box>
    </Tooltip>
  );
}
