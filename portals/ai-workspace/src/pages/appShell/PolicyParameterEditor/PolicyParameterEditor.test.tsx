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

import React from 'react';
import { describe, it, expect, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../test/utils';
import PolicyParameterEditor from './PolicyParameterEditor';
import { ParameterSchema, ParameterValues } from './types';

// Trimmed-down oauth2-generator: token endpoint credentials or a bearer token
const oauth2Parameters: ParameterSchema = {
  type: 'object',
  properties: {
    tokenEndpoint: { type: 'string' },
    clientId: { type: 'string' },
    clientSecret: { type: 'string' },
    bearerToken: { type: 'string' },
  },
  oneOf: [
    { required: ['tokenEndpoint', 'clientId', 'clientSecret'] },
    { required: ['bearerToken'] },
  ],
};

const renderEditor = (
  parameters: ParameterSchema,
  existingValues?: ParameterValues
) =>
  renderWithProviders(
    <PolicyParameterEditor
      policyDefinition={{
        name: 'oauth2-generator',
        version: 'v0.9.0',
        description: '',
        parameters,
      }}
      existingValues={existingValues}
      onCancel={vi.fn()}
      onSubmit={vi.fn()}
    />
  );

describe('oneOf errors', () => {
  it('clearing one conflicting field clears the errors on its siblings', async () => {
    const { user } = renderEditor(oauth2Parameters, {
      tokenEndpoint: 'https://idp/token',
      clientId: 'id',
      clientSecret: 'secret',
      bearerToken: 'static-token',
    });

    await user.click(screen.getByTestId('policy-param-submit'));
    expect(screen.getAllByText(/Cannot be combined/)).toHaveLength(4);

    await user.clear(screen.getByTestId('field-bearerToken'));
    expect(screen.queryByText(/Cannot be combined/)).not.toBeInTheDocument();
  });
});

describe('constraint alerts', () => {
  it('shows the oneOf alert', () => {
    renderEditor(oauth2Parameters);

    expect(
      screen.getByText(
        'Configure exactly one of: tokenEndpoint, clientId and clientSecret or bearerToken.'
      )
    ).toBeInTheDocument();
  });

  it('skips the oneOf alert when options overlap', () => {
    renderEditor({
      type: 'object',
      properties: {
        allowedPhrases: { type: 'array', items: { type: 'string' } },
        deniedPhrases: { type: 'array', items: { type: 'string' } },
      },
      oneOf: [
        { required: ['allowedPhrases'], not: { required: ['deniedPhrases'] } },
        { required: ['deniedPhrases'], not: { required: ['allowedPhrases'] } },
        { required: ['allowedPhrases', 'deniedPhrases'] },
      ],
    });

    expect(screen.queryByText(/exactly one of/)).not.toBeInTheDocument();
  });

  it('shows both the oneOf and the anyOf alert when a schema has both', () => {
    renderEditor({
      ...oauth2Parameters,
      properties: {
        ...oauth2Parameters.properties,
        scope: { type: 'string' },
        audience: { type: 'string' },
      },
      anyOf: [{ required: ['scope'] }, { required: ['audience'] }],
    });

    expect(screen.getByText(/Configure exactly one of/)).toBeInTheDocument();
    expect(
      screen.getByText('At least one of scope, audience must be configured.')
    ).toBeInTheDocument();
  });
});
