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

import { Form, FormControl, FormHelperText, FormLabel, Grid, OutlinedInput, Paper } from '@wso2/oxygen-ui';
import { defineMessages, FormattedMessage, useIntl, type MessageDescriptor } from 'react-intl';

import type { DraftFormField, DraftFormValues, FormFieldErrors } from '../utils/publicationForm';

const messages = defineMessages({
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.nameLabel',
    defaultMessage: 'Name',
  },
  versionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.versionLabel',
    defaultMessage: 'Version',
  },
  descriptionLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.descriptionLabel',
    defaultMessage: 'Description',
  },
  descriptionPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.descriptionPlaceholder',
    defaultMessage: 'What this API does, in one or two sentences.',
  },
  productionUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.productionUrlLabel',
    defaultMessage: 'Production URL',
  },
  sandboxUrlLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.portals.components.ApiDetailsTab.sandboxUrlLabel',
    defaultMessage: 'Sandbox URL',
  },
});

export type ApiDetailsTabProps = {
  disabled?: boolean;
  errors: FormFieldErrors;
  onBlurField: (field: DraftFormField) => void;
  onChange: (values: DraftFormValues) => void;
  values: DraftFormValues;
};

type DetailFieldProps = {
  disabled?: boolean;
  error?: MessageDescriptor;
  field: DraftFormField;
  id: string;
  label: MessageDescriptor;
  onBlur: (field: DraftFormField) => void;
  onChange: (field: DraftFormField, value: string) => void;
  required?: boolean;
  value: string;
};

/** A single-line text field with its label and validation message. */
function DetailField({ disabled, error, field, id, label, onBlur, onChange, required, value }: DetailFieldProps) {
  return (
    <FormControl disabled={disabled} error={Boolean(error)} fullWidth required={required}>
      <FormLabel htmlFor={id}>
        <FormattedMessage {...label} />
      </FormLabel>
      <OutlinedInput
        aria-describedby={`${id}-error`}
        id={id}
        onBlur={() => onBlur(field)}
        onChange={(event) => onChange(field, event.target.value)}
        value={value}
      />
      <FormHelperText id={`${id}-error`}>{error && <FormattedMessage {...error} />}</FormHelperText>
    </FormControl>
  );
}

/**
 * "API Details": name, version, description and the two endpoint URLs in a
 * single card. The thumbnail control and the document picker are left out of
 * this release.
 */
export function ApiDetailsTab({ disabled, errors, onBlurField, onChange, values }: ApiDetailsTabProps) {
  const intl = useIntl();

  const setField = <K extends keyof DraftFormValues>(field: K, value: DraftFormValues[K]) =>
    onChange({ ...values, [field]: value });

  const fieldProps = (field: DraftFormField, id: string, label: MessageDescriptor) => ({
    disabled,
    error: errors[field],
    field,
    id,
    label,
    onBlur: onBlurField,
    onChange: setField,
    value: values[field],
  });

  return (
    <Paper component="section" sx={{ p: 3 }}>
      <Form.Stack spacing={2}>
        <Grid container spacing={2}>
          <Grid size={{ md: 8, xs: 12 }}>
            <DetailField {...fieldProps('displayName', 'publicationDisplayName', messages.nameLabel)} required />
          </Grid>
          <Grid size={{ md: 4, xs: 12 }}>
            <DetailField {...fieldProps('version', 'publicationVersion', messages.versionLabel)} required />
          </Grid>
        </Grid>

        <FormControl disabled={disabled} fullWidth>
          <FormLabel htmlFor="publicationDescription">{intl.formatMessage(messages.descriptionLabel)}</FormLabel>
          <OutlinedInput
            id="publicationDescription"
            multiline
            onChange={(event) => setField('description', event.target.value)}
            placeholder={intl.formatMessage(messages.descriptionPlaceholder)}
            rows={3}
            value={values.description}
          />
        </FormControl>

        <Grid container spacing={2}>
          <Grid size={{ md: 6, xs: 12 }}>
            <DetailField {...fieldProps('productionUrl', 'publicationProductionUrl', messages.productionUrlLabel)} />
          </Grid>
          <Grid size={{ md: 6, xs: 12 }}>
            <DetailField {...fieldProps('sandboxUrl', 'publicationSandboxUrl', messages.sandboxUrlLabel)} />
          </Grid>
        </Grid>
      </Form.Stack>
    </Paper>
  );
}
