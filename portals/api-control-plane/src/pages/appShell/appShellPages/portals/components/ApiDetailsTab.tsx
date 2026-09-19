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
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

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

/**
 * "API Details" — the one form this alpha shows: name, version, description
 * and the two author-entered endpoint URLs, in a single card like the edit-API form. The icon/thumbnail
 * control is left out entirely rather than shown disabled — thumbnails are out
 * of scope for this release, as is the doc-attachment picker.
 */
export function ApiDetailsTab({ disabled, errors, onBlurField, onChange, values }: ApiDetailsTabProps) {
  const intl = useIntl();

  const setField = <K extends keyof DraftFormValues>(field: K, value: DraftFormValues[K]) =>
    onChange({ ...values, [field]: value });

  return (
    <Paper component="section" sx={{ p: 3 }}>
      <Form.Stack spacing={2}>
        <Grid container spacing={2}>
          <Grid size={{ md: 8, xs: 12 }}>
            <FormControl disabled={disabled} error={Boolean(errors.displayName)} fullWidth required>
              <FormLabel htmlFor="publicationDisplayName">{intl.formatMessage(messages.nameLabel)}</FormLabel>
              <OutlinedInput
                aria-describedby="publicationDisplayName-error"
                id="publicationDisplayName"
                onBlur={() => onBlurField('displayName')}
                onChange={(event) => setField('displayName', event.target.value)}
                value={values.displayName}
              />
              <FormHelperText id="publicationDisplayName-error">
                {errors.displayName && <FormattedMessage {...errors.displayName} />}
              </FormHelperText>
            </FormControl>
          </Grid>

          <Grid size={{ md: 4, xs: 12 }}>
            <FormControl disabled={disabled} error={Boolean(errors.version)} fullWidth required>
              <FormLabel htmlFor="publicationVersion">{intl.formatMessage(messages.versionLabel)}</FormLabel>
              <OutlinedInput
                aria-describedby="publicationVersion-error"
                id="publicationVersion"
                onBlur={() => onBlurField('version')}
                onChange={(event) => setField('version', event.target.value)}
                value={values.version}
              />
              <FormHelperText id="publicationVersion-error">
                {errors.version && <FormattedMessage {...errors.version} />}
              </FormHelperText>
            </FormControl>
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
            <FormControl disabled={disabled} error={Boolean(errors.productionUrl)} fullWidth>
              <FormLabel htmlFor="publicationProductionUrl">
                {intl.formatMessage(messages.productionUrlLabel)}
              </FormLabel>
              <OutlinedInput
                aria-describedby="publicationProductionUrl-error"
                id="publicationProductionUrl"
                onBlur={() => onBlurField('productionUrl')}
                onChange={(event) => setField('productionUrl', event.target.value)}
                value={values.productionUrl}
              />
              <FormHelperText id="publicationProductionUrl-error">
                {errors.productionUrl && <FormattedMessage {...errors.productionUrl} />}
              </FormHelperText>
            </FormControl>
          </Grid>

          <Grid size={{ md: 6, xs: 12 }}>
            <FormControl disabled={disabled} error={Boolean(errors.sandboxUrl)} fullWidth>
              <FormLabel htmlFor="publicationSandboxUrl">{intl.formatMessage(messages.sandboxUrlLabel)}</FormLabel>
              <OutlinedInput
                aria-describedby="publicationSandboxUrl-error"
                id="publicationSandboxUrl"
                onBlur={() => onBlurField('sandboxUrl')}
                onChange={(event) => setField('sandboxUrl', event.target.value)}
                value={values.sandboxUrl}
              />
              <FormHelperText id="publicationSandboxUrl-error">
                {errors.sandboxUrl && <FormattedMessage {...errors.sandboxUrl} />}
              </FormHelperText>
            </FormControl>
          </Grid>
        </Grid>
      </Form.Stack>
    </Paper>
  );
}
