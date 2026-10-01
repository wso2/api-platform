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
 * Templates and widgets for react-jsonschema-form that follow Oxygen UI form
 * conventions: labels above fields, plain-text help, outlined item cards, and
 * an "Advanced settings" section. They know nothing about policies; what counts
 * as advanced comes from `formContext.isAdvancedProperty`.
 */
import React, { useState } from 'react';
import {
  Alert,
  Box,
  Button,
  Collapse,
  FormControl,
  FormLabel,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronRight, Plus } from '@wso2/oxygen-ui-icons-react';
import { Templates as MuiTemplates, Widgets as MuiWidgets } from '@rjsf/mui';
import { buttonId, descriptionId, getTemplate, getUiOptions, titleId } from '@rjsf/utils';
import type {
  ArrayFieldItemTemplateProps,
  ArrayFieldTemplateProps,
  BaseInputTemplateProps,
  ErrorListProps,
  DescriptionFieldProps,
  ErrorSchema,
  FieldTemplateProps,
  ObjectFieldTemplateProps,
  OptionalDataControlsTemplateProps,
  TitleFieldProps,
  WidgetProps,
} from '@rjsf/utils';

/** Form context this theme reads. */
export interface OxygenFormContext {
  /** Returns true for a property schema that belongs in "Advanced settings". */
  isAdvancedProperty?: (propertySchema: unknown) => boolean;
}

function hasErrors(errorSchema: ErrorSchema | undefined): boolean {
  if (!errorSchema) {
    return false;
  }
  return Object.entries(errorSchema).some(([key, value]) =>
    key === '__errors'
      ? Array.isArray(value) && value.length > 0
      : hasErrors(value as ErrorSchema)
  );
}

export function FieldTemplate(props: FieldTemplateProps) {
  const {
    id,
    children,
    classNames,
    style,
    disabled,
    displayLabel,
    hidden,
    label,
    onKeyRename,
    onKeyRenameBlur,
    onRemoveProperty,
    readonly,
    required,
    rawErrors = [],
    errors,
    help,
    rawDescription,
    schema,
    uiSchema,
    registry,
  } = props;
  const uiOptions = getUiOptions(uiSchema);
  const WrapIfAdditionalTemplate = getTemplate('WrapIfAdditionalTemplate', registry, uiOptions);

  if (hidden) {
    return <div style={{ display: 'none' }}>{children}</div>;
  }

  // Checkboxes render their own label next to the box.
  const isCheckbox = uiOptions.widget === 'checkbox' || schema.type === 'boolean';
  const showLabel = displayLabel && !isCheckbox && !!label;

  return (
    <WrapIfAdditionalTemplate
      classNames={classNames}
      style={style}
      disabled={disabled}
      id={id}
      label={label}
      displayLabel={displayLabel}
      rawDescription={rawDescription}
      onKeyRename={onKeyRename}
      onKeyRenameBlur={onKeyRenameBlur}
      onRemoveProperty={onRemoveProperty}
      readonly={readonly}
      required={required}
      schema={schema}
      uiSchema={uiSchema}
      registry={registry}
    >
      <FormControl fullWidth error={rawErrors.length > 0} required={required}>
        {showLabel && (
          <FormLabel htmlFor={id} sx={{ mb: 0.5 }}>
            {label}
          </FormLabel>
        )}
        {children}
        {displayLabel && !isCheckbox && rawDescription ? (
          <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5 }}>
            {rawDescription}
          </Typography>
        ) : null}
        {errors}
        {help}
      </FormControl>
    </WrapIfAdditionalTemplate>
  );
}

// The label is rendered above the field by FieldTemplate, so inputs don't get
// MUI's floating label as well.
const MuiBaseInputTemplate = MuiTemplates.BaseInputTemplate!;
export function BaseInputTemplate(props: BaseInputTemplateProps) {
  return <MuiBaseInputTemplate {...props} hideLabel />;
}

const MuiSelectWidget = MuiWidgets.SelectWidget!;
export function SelectWidget(props: WidgetProps) {
  return <MuiSelectWidget {...props} hideLabel />;
}

const MuiRadioWidget = MuiWidgets.RadioWidget!;
export function RadioWidget(props: WidgetProps) {
  return <MuiRadioWidget {...props} hideLabel />;
}

export function TitleFieldTemplate(props: TitleFieldProps) {
  const { id, title, optionalDataControl } = props;
  return (
    <Box
      id={id}
      sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, mt: 1, mb: 0.5 }}
    >
      <Typography variant="body1" sx={{ fontWeight: 600 }}>
        {title}
      </Typography>
      {optionalDataControl}
    </Box>
  );
}

export function DescriptionFieldTemplate(props: DescriptionFieldProps) {
  const { id, description } = props;
  if (!description) {
    return null;
  }
  // Plain text, like field help, so descriptions never render Markdown or HTML.
  return (
    <Typography id={id} variant="caption" component="div" color="text.secondary" sx={{ mb: 0.5 }}>
      {description}
    </Typography>
  );
}

export function ObjectFieldTemplate(props: ObjectFieldTemplateProps) {
  const {
    description,
    title,
    properties,
    required,
    disabled,
    readonly,
    uiSchema,
    fieldPathId,
    schema,
    errorSchema,
    optionalDataControl,
    registry,
  } = props;
  const [showAdvanced, setShowAdvanced] = useState(false);
  const uiOptions = getUiOptions(uiSchema);
  const TitleField = getTemplate('TitleFieldTemplate', registry, uiOptions);
  const DescriptionField = getTemplate('DescriptionFieldTemplate', registry, uiOptions);
  const showControlInTitle = !readonly && !disabled;

  const { isAdvancedProperty } = (registry.formContext ?? {}) as OxygenFormContext;
  const propertySchemas = (schema.properties ?? {}) as Record<string, unknown>;
  const main = properties.filter((p) => !isAdvancedProperty?.(propertySchemas[p.name]));
  const advanced = properties.filter((p) => isAdvancedProperty?.(propertySchemas[p.name]));
  // Open the section when a field in it has an error, so the error is visible.
  const advancedOpen =
    showAdvanced ||
    advanced.some((p) => hasErrors((errorSchema as Record<string, ErrorSchema> | undefined)?.[p.name]));

  // Array items are shown as cards, so their generated titles ("Questions-1") are left out.
  const isArrayItem = typeof fieldPathId.path[fieldPathId.path.length - 1] === 'number';
  const showTitle = !!title && !isArrayItem;

  const render = (items: typeof properties) =>
    items.map((p) => (p.hidden ? <React.Fragment key={p.name}>{p.content}</React.Fragment> : <Box key={p.name}>{p.content}</Box>));

  return (
    <Box>
      {showTitle && (
        <TitleField
          id={titleId(fieldPathId)}
          title={title}
          required={required}
          schema={schema}
          uiSchema={uiSchema}
          registry={registry}
          optionalDataControl={showControlInTitle ? optionalDataControl : undefined}
        />
      )}
      {description && (
        <DescriptionField
          id={descriptionId(fieldPathId)}
          description={description}
          schema={schema}
          uiSchema={uiSchema}
          registry={registry}
        />
      )}
      {!showControlInTitle ? optionalDataControl : undefined}
      <Stack spacing={2} sx={{ mt: properties.length > 0 && (showTitle || description) ? 1 : 0 }}>
        {render(main)}
        {advanced.length > 0 && (
          <Box>
            <Button
              size="small"
              variant="text"
              onClick={() => setShowAdvanced(!advancedOpen)}
              startIcon={advancedOpen ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
            >
              Advanced settings
            </Button>
            <Collapse in={advancedOpen}>
              <Stack spacing={2} sx={{ mt: 1 }}>
                {render(advanced)}
              </Stack>
            </Collapse>
          </Box>
        )}
      </Stack>
    </Box>
  );
}

export function ArrayFieldTemplate(props: ArrayFieldTemplateProps) {
  const {
    canAdd,
    disabled,
    fieldPathId,
    uiSchema,
    items,
    optionalDataControl,
    onAddClick,
    readonly,
    registry,
    required,
    schema,
    title,
  } = props;
  const uiOptions = getUiOptions(uiSchema);
  const ArrayFieldTitle = getTemplate('ArrayFieldTitleTemplate', registry, uiOptions);
  const ArrayFieldDescription = getTemplate('ArrayFieldDescriptionTemplate', registry, uiOptions);
  const showControlInTitle = !readonly && !disabled;
  const { AddButton } = registry.templates.ButtonTemplates;

  return (
    <Box>
      <ArrayFieldTitle
        fieldPathId={fieldPathId}
        title={uiOptions.title || title}
        schema={schema}
        uiSchema={uiSchema}
        required={required}
        registry={registry}
        optionalDataControl={showControlInTitle ? optionalDataControl : undefined}
      />
      <ArrayFieldDescription
        fieldPathId={fieldPathId}
        description={uiOptions.description || schema.description}
        schema={schema}
        uiSchema={uiSchema}
        registry={registry}
      />
      {!showControlInTitle ? optionalDataControl : undefined}
      <Box sx={{ mt: 1 }}>{items}</Box>
      {canAdd && (
        <AddButton
          id={buttonId(fieldPathId, 'add')}
          className="rjsf-array-item-add"
          onClick={onAddClick}
          disabled={disabled || readonly}
          uiSchema={uiSchema}
          registry={registry}
        />
      )}
    </Box>
  );
}

export function ArrayFieldItemTemplate(props: ArrayFieldItemTemplateProps) {
  const { children, buttonsProps, hasToolbar, uiSchema, registry } = props;
  const uiOptions = getUiOptions(uiSchema);
  const ArrayFieldItemButtons = getTemplate('ArrayFieldItemButtonsTemplate', registry, uiOptions);
  // Objects get an outlined card; single values stay a plain row.
  const isObject = buttonsProps.schema?.type === 'object';
  return (
    <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-start', mb: 1.5 }}>
      <Box
        sx={
          isObject
            ? { flex: 1, minWidth: 0, border: 1, borderColor: 'divider', borderRadius: 1, p: 2 }
            : { flex: 1, minWidth: 0 }
        }
      >
        {children}
      </Box>
      {hasToolbar && (
        <Box sx={{ display: 'flex', flexDirection: isObject ? 'column' : 'row', pt: isObject ? 1 : 0.5 }}>
          <ArrayFieldItemButtons {...buttonsProps} />
        </Box>
      )}
    </Box>
  );
}

export function OptionalDataControlsTemplate(props: OptionalDataControlsTemplateProps) {
  const { id, label, onAddClick, onRemoveClick } = props;
  if (onAddClick) {
    return (
      <Button id={id} size="small" variant="outlined" startIcon={<Plus size={16} />} onClick={onAddClick}>
        {label}
      </Button>
    );
  }
  if (onRemoveClick) {
    return (
      <Button id={id} size="small" variant="text" color="error" onClick={onRemoveClick}>
        {label}
      </Button>
    );
  }
  return (
    <Typography id={id} variant="body2" color="text.secondary">
      {label}
    </Typography>
  );
}

export function ErrorListTemplate(props: ErrorListProps) {
  // The same message can come from both the form's own check and a later check
  // (for example, against the policy's parameters), so list each one once.
  const seen = new Set<string>();
  const errors = props.errors.filter((error) => {
    const text = `${error.property}|${error.message}`;
    if (seen.has(text)) {
      return false;
    }
    seen.add(text);
    return true;
  });
  return (
    <Alert severity="error" sx={{ mb: 2 }}>
      <Typography variant="body2" sx={{ fontWeight: 600, mb: 0.5 }}>
        Fix these before saving:
      </Typography>
      <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
        {errors.map((error, i) => (
          <li key={i}>
            <Typography variant="body2">
              {error.title ? `${error.title}: ${error.message}` : error.message}
            </Typography>
          </li>
        ))}
      </Box>
    </Alert>
  );
}
