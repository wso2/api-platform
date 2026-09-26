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

import { TranslatableString, englishStringTranslator } from '@rjsf/utils';
import type { ErrorTransformer, RJSFValidationError } from '@rjsf/utils';
import { ParameterValues } from '../types';

function limit(error: RJSFValidationError): unknown {
  return (error.params as { limit?: unknown } | undefined)?.limit;
}

function friendlyMessage(error: RJSFValidationError): string | undefined {
  switch (error.name) {
    case 'required':
      return 'This field is required.';
    case 'minLength':
      return limit(error) === 1 ? 'This field is required.' : `Enter at least ${limit(error)} characters.`;
    case 'maxLength':
      return `Enter at most ${limit(error)} characters.`;
    case 'minimum':
      return `Enter a value of at least ${limit(error)}.`;
    case 'maximum':
      return `Enter a value of at most ${limit(error)}.`;
    case 'exclusiveMinimum':
      return `Enter a value greater than ${limit(error)}.`;
    case 'exclusiveMaximum':
      return `Enter a value less than ${limit(error)}.`;
    case 'minItems':
      return `Add at least ${limit(error)}.`;
    case 'maxItems':
      return `Add at most ${limit(error)}.`;
    case 'uniqueItems':
      return 'Each entry must be different.';
    case 'enum':
      return 'Pick one of the listed values.';
    case 'type':
      return `Enter a ${(error.params as { type?: string } | undefined)?.type ?? 'valid'} value.`;
    case 'pattern':
      return 'This value is not in the expected format.';
    case 'anyOf':
      return 'Configure at least one of the sections named in the note above.';
    default:
      return undefined;
  }
}

/**
 * Makes Ajv's messages readable. Messages a formSchema sets with `errorMessage`
 * are kept as written. Ajv's "must match then schema" errors repeat the real
 * error, and the per-branch errors of an anyOf repeat the anyOf error, so both
 * are dropped.
 */
export const transformErrors: ErrorTransformer<ParameterValues> = (errors) =>
  errors
    .filter((e) => e.name !== 'if' && !(e.schemaPath ?? '').includes('/anyOf/'))
    .map((e) => {
      const message = friendlyMessage(e);
      return message ? { ...e, message, stack: `${e.property ?? ''} ${message}`.trim() } : e;
    })
    .map(attachRequiredMessageToField);

/**
 * An `errorMessage` for a missing field (`errorMessage: { required: { scope: ... } }`)
 * is reported on the parent object. Move it to the missing field, so it shows
 * under that field like any other error.
 */
function attachRequiredMessageToField(error: RJSFValidationError): RJSFValidationError {
  if (error.name !== 'errorMessage') {
    return error;
  }
  const original = (error.params as { errors?: Array<{ keyword?: string; params?: { missingProperty?: string } }> } | undefined)
    ?.errors;
  const missing = original?.length === 1 && original[0].keyword === 'required' ? original[0].params?.missingProperty : undefined;
  if (!missing) {
    return error;
  }
  const property = `${error.property ?? ''}.${missing}`;
  return { ...error, property, title: undefined, stack: `${property} ${error.message ?? ''}`.trim() };
}

/** Labels for RJSF's built-in buttons and messages. */
export function translateString(str: TranslatableString, params?: string[]): string {
  switch (str) {
    case TranslatableString.OptionalObjectAdd:
      return 'Turn on';
    case TranslatableString.OptionalObjectRemove:
      return 'Turn off';
    case TranslatableString.OptionalObjectEmptyMsg:
      return 'Not configured';
    case TranslatableString.AddItemButton:
      return 'Add';
    default:
      return englishStringTranslator(str, params);
  }
}
