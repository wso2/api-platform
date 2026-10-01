/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC. and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein is strictly forbidden, unless permitted by WSO2 in accordance with
 * the WSO2 Commercial License available at http://wso2.com/licenses.
 * For specific language governing the permissions and limitations under
 * this license, please see the license as well as any agreement you've
 * entered into with WSO2 governing the purchase of this software and any
 * associated services.
 */

/*
 * Button templates for react-jsonschema-form, using Oxygen UI buttons and the
 * lucide icons the app already uses.
 */
import React from 'react';
import { Button, IconButton } from '@wso2/oxygen-ui';
import {
  ChevronDown,
  ChevronUp,
  Copy,
  Plus,
  Trash2,
  X,
} from '@wso2/oxygen-ui-icons-react';
import { TranslatableString } from '@rjsf/utils';
import type { IconButtonProps, SubmitButtonProps } from '@rjsf/utils';

type IconComponent = React.ComponentType<{ size?: number }>;

function iconButton(Icon: IconComponent, label: TranslatableString, color?: 'error') {
  function OxygenIconButton(props: IconButtonProps) {
    // `icon`, `iconType`, `uiSchema` and `registry` are RJSF props, not DOM props.
    const { icon: _icon, iconType: _iconType, uiSchema: _uiSchema, registry, color: _color, ...buttonProps } = props;
    return (
      <IconButton
        size="small"
        title={registry.translateString(label)}
        color={color}
        {...buttonProps}
      >
        <Icon size={16} />
      </IconButton>
    );
  }
  return OxygenIconButton;
}

export const CopyButton = iconButton(Copy, TranslatableString.CopyButton);
export const MoveDownButton = iconButton(ChevronDown, TranslatableString.MoveDownButton);
export const MoveUpButton = iconButton(ChevronUp, TranslatableString.MoveUpButton);
export const RemoveButton = iconButton(Trash2, TranslatableString.RemoveButton, 'error');
export const ClearButton = iconButton(X, TranslatableString.ClearButton);

export function AddButton(props: IconButtonProps) {
  const { icon: _icon, iconType: _iconType, uiSchema: _uiSchema, registry, color: _color, ...buttonProps } = props;
  return (
    <Button size="small" variant="outlined" startIcon={<Plus size={16} />} {...buttonProps}>
      {registry.translateString(TranslatableString.AddItemButton)}
    </Button>
  );
}

// Forms built on this theme submit from their own buttons.
export function SubmitButton(_props: SubmitButtonProps) {
  return null;
}
