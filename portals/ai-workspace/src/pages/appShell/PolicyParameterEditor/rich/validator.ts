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

import ajvErrors from 'ajv-errors';
import { customizeValidator } from '@rjsf/validator-ajv8';
import { getByPath, toPath } from '@rjsf/utils';
import type {
  CustomValidator,
  ErrorTransformer,
  RJSFSchema,
  UiSchema,
  ValidationData,
  ValidatorType,
} from '@rjsf/utils';
import { isTemplateExpression } from '../schemaUtils';
import { ParameterValues } from '../types';

// ajv-errors adds the `errorMessage` keyword that formSchemas use for their own
// messages. RJSF's Ajv defaults already include the `allErrors` it needs, and
// strict mode is off, so vendor keywords such as x-wso2-* are ignored.
const ajvValidator = customizeValidator<ParameterValues>({ extenderFn: ajvErrors });

function valueAt(data: unknown, property: string | undefined): unknown {
  if (!property || data === null || typeof data !== 'object') {
    return undefined;
  }
  return getByPath(data as object, toPath(property));
}

/**
 * Wraps the Ajv validator so that values holding a template expression, such as
 * {{ env "LIMIT" }} or {{ secret "key" }}, aren't flagged: the gateway resolves
 * them before it validates, so their final type and format aren't known here.
 */
class TemplateAwareValidator implements ValidatorType<ParameterValues> {
  constructor(private readonly inner: ValidatorType<ParameterValues>) {}

  validateFormData(
    formData: ParameterValues | undefined,
    schema: RJSFSchema,
    customValidate?: CustomValidator<ParameterValues>,
    transformErrors?: ErrorTransformer<ParameterValues>,
    uiSchema?: UiSchema<ParameterValues>
  ): ValidationData<ParameterValues> {
    const skipTemplateValues: ErrorTransformer<ParameterValues> = (errors, ui) => {
      const kept = errors.filter((e) => !isTemplateExpression(valueAt(formData, e.property)));
      return transformErrors ? transformErrors(kept, ui) : kept;
    };
    return this.inner.validateFormData(formData, schema, customValidate, skipTemplateValues, uiSchema);
  }

  isValid(schema: RJSFSchema, formData: ParameterValues | undefined, rootSchema: RJSFSchema): boolean {
    return this.inner.isValid(schema, formData, rootSchema);
  }

  rawValidation<Result = unknown>(schema: RJSFSchema, formData?: ParameterValues) {
    return this.inner.rawValidation<Result>(schema, formData);
  }

  reset() {
    this.inner.reset?.();
  }
}

/** The validator for rich policy forms, shared so Ajv's compiled schemas are reused. */
export const policyValidator = new TemplateAwareValidator(ajvValidator);

/**
 * Throws if Ajv can't compile the schema, or if it declares a JSON Schema draft
 * other than draft-07 (Ajv's default here).
 */
export function preflight(schema: RJSFSchema): void {
  if (schema.$schema && !String(schema.$schema).includes('draft-07')) {
    throw new Error(`Unsupported JSON Schema draft: ${schema.$schema}`);
  }
  const { validationError } = ajvValidator.rawValidation(schema, {});
  if (validationError) {
    throw validationError;
  }
}
