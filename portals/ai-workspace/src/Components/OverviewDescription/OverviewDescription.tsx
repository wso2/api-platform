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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { useLayoutEffect, useRef, useState } from 'react';
import { Tooltip, Typography } from '@wso2/oxygen-ui';

import { OVERVIEW_DESCRIPTION_SX, OVERVIEW_DESCRIPTION_TOOLTIP_WIDTH } from '../../utils/overviewHeaderStyles';

type OverviewDescriptionProps = {
  text: string;
};

/** An Overview header's description, cut to a few lines; the full text is in a tooltip when it is cut. */
export default function OverviewDescription({ text }: OverviewDescriptionProps) {
  const ref = useRef<HTMLParagraphElement>(null);
  const [isClamped, setIsClamped] = useState(false);
  useLayoutEffect(() => {
    const element = ref.current;
    if (!element) return undefined;
    const measure = () => setIsClamped(element.scrollHeight > element.clientHeight + 1);
    measure();
    if (typeof ResizeObserver === 'undefined') return undefined;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, [text]);

  return (
    <Tooltip
      slotProps={{ tooltip: { sx: { maxWidth: OVERVIEW_DESCRIPTION_TOOLTIP_WIDTH } } }}
      title={isClamped ? text : ''}
    >
      <Typography color="text.secondary" ref={ref} sx={OVERVIEW_DESCRIPTION_SX} variant="body2">
        {text}
      </Typography>
    </Tooltip>
  );
}
