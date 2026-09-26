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

import React, { useCallback, useMemo, useRef, useState } from 'react';
import { Box } from '@wso2/oxygen-ui';
import type Form from '@rjsf/core';
import type { ErrorSchema, RJSFSchema } from '@rjsf/utils';
import { OxygenForm } from '../../../../Components/RjsfOxygen';
import type { OxygenFormContext } from '../../../../Components/RjsfOxygen';
import { PolicyEditorActions, PolicyEditorHeader } from '../PolicyEditorChrome';
import type { PolicyParameterEditorProps } from '../PolicyParameterEditor';
import { omitOptionalEmptyValues } from '../schemaUtils';
import { useStyles } from '../styles';
import { ParameterValues } from '../types';
import { transformErrors, translateString } from './errorMessages';
import { buildUiSchema } from './sanitizeUiSchema';
import { policyValidator, preflight } from './validator';

// Only defaults of required fields are pre-filled, so optional sections (such as
// content safety's response screening) stay off until the user turns them on,
// and new array items get the defaults of the fields their schema requires.
const DEFAULT_FORM_STATE_BEHAVIOR = {
  emptyObjectFields: 'populateRequiredDefaults',
  arrayMinItems: { populate: 'requiredOnly' },
} as const;

const FORM_CONTEXT: OxygenFormContext = {
  isAdvancedProperty: (propertySchema) =>
    typeof propertySchema === 'object' &&
    propertySchema !== null &&
    (propertySchema as Record<string, unknown>)['x-wso2-policy-advanced-param'] === true,
};

/**
 * Renders a policy's params form from the optional `x-wso2-policy-ui` block with
 * react-jsonschema-form. Takes the same props, and produces the same params
 * object, as PolicyParameterEditor. Before saving, the output is also checked
 * against the policy's `parameters`, which is what the gateway validates.
 */
const RichPolicyForm: React.FC<PolicyParameterEditorProps> = ({
  policyDefinition,
  policyDisplayName,
  existingValues,
  onCancel,
  onSubmit,
  disabled = false,
  readOnly = false,
}) => {
  const classes = useStyles();
  const { name, description, parameters, ui } = policyDefinition;
  const formSchema = ui?.formSchema as RJSFSchema;
  const parametersSchema = parameters as unknown as RJSFSchema;

  // Throws for a schema Ajv can't compile; PolicyEditor then shows the standard editor.
  useMemo(() => {
    preflight(formSchema);
    preflight(parametersSchema);
  }, [formSchema, parametersSchema]);

  const uiSchema = useMemo(() => buildUiSchema(ui?.uiSchema, readOnly), [ui?.uiSchema, readOnly]);
  const formRef = useRef<Form<ParameterValues>>(null);
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const [extraErrors, setExtraErrors] = useState<ErrorSchema<ParameterValues>>();

  const handleSubmit = useCallback(() => {
    const form = formRef.current;
    if (!form) {
      return;
    }
    setSubmitAttempted(true);
    if (!form.validateForm()) {
      return;
    }
    // Drop values the form no longer shows, such as a question's options after
    // its type changed, then empty optional values, as the standard editor does.
    const data = (form.omitExtraData(form.state.formData) ?? {}) as ParameterValues;
    const output = omitOptionalEmptyValues(parameters, data);
    const { errors, errorSchema } = policyValidator.validateFormData(
      output,
      parametersSchema,
      undefined,
      transformErrors
    );
    if (errors.length > 0) {
      setExtraErrors(errorSchema);
      return;
    }
    setExtraErrors(undefined);
    onSubmit(output);
  }, [parameters, parametersSchema, onSubmit]);

  return (
    <Box sx={classes.root}>
      <PolicyEditorHeader
        title={policyDisplayName || name}
        description={description}
        parameters={parameters}
      />
      <OxygenForm
        ref={formRef}
        schema={formSchema}
        uiSchema={uiSchema}
        initialFormData={existingValues}
        validator={policyValidator}
        experimental_defaultFormStateBehavior={DEFAULT_FORM_STATE_BEHAVIOR}
        transformErrors={transformErrors}
        translateString={translateString}
        extraErrors={extraErrors}
        liveValidate={submitAttempted ? 'onChange' : undefined}
        showErrorList="top"
        focusOnFirstError
        noHtml5Validate
        readonly={readOnly}
        disabled={disabled}
        formContext={FORM_CONTEXT}
        // The errors are shown in the form; without a handler RJSF also logs them.
        onError={() => undefined}
        onChange={() => {
          if (extraErrors) {
            setExtraErrors(undefined);
          }
        }}
      />
      <PolicyEditorActions
        onCancel={onCancel}
        onSubmit={handleSubmit}
        isEditing={!!existingValues}
        disabled={disabled}
        readOnly={readOnly}
      />
    </Box>
  );
};

export default RichPolicyForm;
