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

import { Box, Button } from '@wso2/oxygen-ui';
import { useEffect, useState } from 'react';

import { APP_FOOTER_ID } from '../../../layouts/appLayoutConstants';

/**
 * Upward elevation shadow for the bottom action bar. Kept as a named token
 * (theme.shadows are all downward) rather than an inline magic value.
 */
const SAVE_BAR_SHADOW = '0 -2px 10px rgba(0, 0, 0, 0.16)';

/** Static styles for the sticky save bar, kept out of JSX (theme-token based). */
const saveBarBaseSx = {
  // alignItems: 'center',
  // Solid surface (palette token) so scrolling content never shows through.
  borderColor: 'divider',
  borderTop: '1px solid',
  boxShadow: SAVE_BAR_SHADOW,
  display: 'flex',
  gap: 1,
  justifyContent: 'flex-end',
  mt: 1,
  position: 'sticky',
  py: 1.5,
  backdropFilter: 'blur(10px)',
} as const;

type SaveBarProps = {
  /** Disables the save button (invalid form or in-flight save). */
  disabled?: boolean;
  /** Renders the in-flight label and is implied disabled. */
  saving?: boolean;
  onSave: () => void;
  label?: string;
};

/**
 * Measures the app footer's height (it sits at the bottom of the same scroll
 * area this bar sticks to). Returns 0 when the footer is absent (e.g. in tests).
 */
function useFooterHeight(): number {
  const [height, setHeight] = useState(0);
  useEffect(() => {
    const el = document.getElementById(APP_FOOTER_ID);
    if (!el) return;
    const update = () => setHeight(el.offsetHeight);
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  return height;
}

/**
 * Solid save action bar pinned to the bottom of a develop tab's scroll area
 * (`position: sticky`), offset above the app footer so it is never covered.
 */
export function SaveBar({ disabled, saving, onSave, label = 'Save changes' }: SaveBarProps) {
  const footerHeight = useFooterHeight();
  return (
    <Box
      sx={[
        saveBarBaseSx,
        { bottom: `${footerHeight}px` },
        (theme) => ({ zIndex: theme.zIndex.appBar }),
      ]}
    >
      <Button disabled={disabled || saving} onClick={onSave} variant="contained">
        {saving ? 'Saving…' : label}
      </Button>
    </Box>
  );
}
