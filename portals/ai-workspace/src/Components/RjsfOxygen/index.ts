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

import { withTheme } from '@rjsf/core';
import type { ThemeProps } from '@rjsf/core';
import { Templates as MuiTemplates, Widgets as MuiWidgets } from '@rjsf/mui';
import {
  AddButton,
  ClearButton,
  CopyButton,
  MoveDownButton,
  MoveUpButton,
  RemoveButton,
  SubmitButton,
} from './buttons';
import {
  ArrayFieldItemTemplate,
  ArrayFieldTemplate,
  BaseInputTemplate,
  DescriptionFieldTemplate,
  ErrorListTemplate,
  FieldTemplate,
  ObjectFieldTemplate,
  OptionalDataControlsTemplate,
  RadioWidget,
  SelectWidget,
  TitleFieldTemplate,
} from './templates';

export type { OxygenFormContext } from './templates';

/**
 * A react-jsonschema-form theme for Oxygen UI: @rjsf/mui (Oxygen UI is themed
 * MUI) with the templates, widgets and buttons that differ overridden.
 */
export const oxygenTheme: ThemeProps = {
  templates: {
    ...MuiTemplates,
    ArrayFieldItemTemplate,
    ArrayFieldTemplate,
    BaseInputTemplate,
    DescriptionFieldTemplate,
    ErrorListTemplate,
    FieldTemplate,
    ObjectFieldTemplate,
    OptionalDataControlsTemplate,
    TitleFieldTemplate,
    ButtonTemplates: {
      ...MuiTemplates.ButtonTemplates,
      AddButton,
      ClearButton,
      CopyButton,
      MoveDownButton,
      MoveUpButton,
      RemoveButton,
      SubmitButton,
    },
  },
  widgets: {
    ...MuiWidgets,
    RadioWidget,
    SelectWidget,
  },
};

/** A react-jsonschema-form `Form` rendered with Oxygen UI. */
export const OxygenForm = withTheme(oxygenTheme);
