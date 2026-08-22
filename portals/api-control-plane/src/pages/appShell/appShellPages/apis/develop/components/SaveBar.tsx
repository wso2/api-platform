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

import { useFooterHeight } from '../../../../useFooterHeight';
import { stickyBottomBarSx } from '../../../../../../theme';

/** Layout for the save bar's own content; the sticky treatment is shared. */
const saveBarLayoutSx = {
  display: 'flex',
  gap: 1,
  justifyContent: 'flex-end',
  mt: 1,
  py: 1.5,
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
 * Save action bar pinned to the bottom of a develop tab's scroll area
 * (`position: sticky`), offset above the app footer so it is never covered.
 */
export function SaveBar({ disabled, saving, onSave, label = 'Save changes' }: SaveBarProps) {
  const footerHeight = useFooterHeight();
  return (
    <Box
      sx={[
        stickyBottomBarSx,
        saveBarLayoutSx,
        { bottom: `${footerHeight}px` },
      ]}
    >
      <Button disabled={disabled || saving} onClick={onSave} variant="contained">
        {saving ? 'Saving…' : label}
      </Button>
    </Box>
  );
}
