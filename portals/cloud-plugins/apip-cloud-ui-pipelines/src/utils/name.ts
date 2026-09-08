/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

/**
 * A pipeline's name becomes its OpenChoreo resource name, so it has to be a
 * DNS-1123 label: the backend rejects anything else. Checking it here means the
 * rule is visible while the name is being typed, instead of the create failing
 * after the whole pipeline has been built.
 */

/** The longest name Kubernetes accepts for a single object. */
const MAX_PIPELINE_NAME_LENGTH = 63;

const DNS1123_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/;

/**
 * Returns the message to show for `name`, or `undefined` when it is usable. An
 * empty name is left to the submit button's own disabled state rather than
 * reported as an error the moment the field is focused.
 */
export function validatePipelineName(name: string): string | undefined {
  const trimmed = name.trim();
  if (!trimmed) return undefined;

  if (trimmed.length > MAX_PIPELINE_NAME_LENGTH) {
    return `Use at most ${MAX_PIPELINE_NAME_LENGTH} characters.`;
  }
  if (!DNS1123_LABEL.test(trimmed)) {
    return 'Use only lowercase letters, numbers and hyphens, starting and ending with a letter or number.';
  }
  return undefined;
}
