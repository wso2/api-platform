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

import { Box, Stack, Typography } from '@wso2/oxygen-ui';
import { useCallback, useState } from 'react';
import { defineMessages, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';
import { DefineApiPanel } from './components/DefineApiPanel';
import { GeneralCreateApiForm } from './components/GeneralCreateApiForm';
import { ApiCreationWizardDraftState, ApiType, GeneralApiCreationFormState } from './types';
import { ApiTypeSelector } from './components/ApiTypeSelector';
import { ApiCreationStepKey, ApiCreationSteps } from './components/ApiCreationSteps';
import { useCreateRestApi } from '@/api/resources/restApis';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { routes } from '@/routes/paths';
import { toCreateRestApiBody } from './utils/createRestApiBody';
import { toCreateApiFormErrors, type CreateApiFormErrors } from './utils/serverFieldErrors';
import {
  ApiCreationProgress,
  type ApiCreationProgressStatus,
} from './components/ApiCreationProgress';

const messages = defineMessages({
  apiTypeSubtitle: {
    id: 'api.create.ApiCreationWizard.apiType.subtitle',
    defaultMessage:
      'This decides how the gateway exposes your backend. Only REST is available today.',
  },
  apiTypeTitle: {
    id: 'api.create.ApiCreationWizard.apiType.title',
    defaultMessage: 'What kind of API are you exposing?',
  },
  configureSubtitle: {
    id: 'api.create.ApiCreationWizard.configure.subtitle',
    defaultMessage: 'Name it, set where it routes, and create it.',
  },
  configureTitle: {
    id: 'api.create.ApiCreationWizard.configure.title',
    defaultMessage: 'Configure and create',
  },
  genericApiType: {
    id: 'api.create.ApiCreationWizard.apiType.generic',
    defaultMessage: 'API',
    description:
      'Stands in for the chosen API type when the wizard is opened straight at a later step.',
  },
  sourceSubtitle: {
    id: 'api.create.ApiCreationWizard.source.subtitle',
    defaultMessage:
      'Bring an existing contract, or start from a blank slate and fill in the details yourself.',
  },
  sourceTitle: {
    id: 'api.create.ApiCreationWizard.source.title',
    defaultMessage: 'How do you want to define your {apiType}?',
    description:
      '{apiType} is the type picked in the first step, e.g. "REST API". Reads as one sentence.',
  },
});

export const ApiCreationWizard = () => {
  const intl = useIntl();
  const [step, setStep] = useState<ApiCreationStepKey>('apiType');
  const [apiType, setApiType] = useState<ApiType | null>(null);

  const [prefilledData, setPrefilledData] = useState<Partial<GeneralApiCreationFormState>>({});

  /**
   * The chosen type's own name, translated. `apiType` is already the entry from
   * the catalog, so its descriptor is read directly rather than looked up again.
   */
  const apiTypeName =
    apiType === null
      ? intl.formatMessage(messages.genericApiType)
      : intl.formatMessage(apiType.title);

  const getTitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return intl.formatMessage(messages.apiTypeTitle);
      case 'source':
        return intl.formatMessage(messages.sourceTitle, { apiType: apiTypeName });
      case 'configure':
        return intl.formatMessage(messages.configureTitle);
      default:
        return '';
    }
  };

  const getSubtitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return intl.formatMessage(messages.apiTypeSubtitle);
      case 'source':
        return intl.formatMessage(messages.sourceSubtitle);
      case 'configure':
        return intl.formatMessage(messages.configureSubtitle);
      default:
        return '';
    }
  };

  /**
   * Replaces the draft rather than merging into it: `extractApiDetails` omits
   * the keys a document doesn't answer for, so spreading over the previous
   * draft would carry an earlier import's fields into a later one.
   *
   * A fresh import also supersedes any earlier submission, so the form starts
   * from the new document rather than restoring values typed against the old.
   */
  const handleDataExtracted = (data: ApiCreationWizardDraftState) => {
    setPrefilledData(data);
    setSubmittedValues(null);

    setStep('configure');
  };

  const navigate = useNavigate();
  // `handlesErrors`: a rejection this screen puts back on the form must not
  // also arrive as a snackbar that has faded by the time the user looks up.
  const createRestApiMutation = useCreateRestApi({ handlesErrors: true });
  // `projectId` on the request body is the project handle from the route, not
  // something the form collects.
  const { activeScope, params } = useConsoleScope();

  /**
   * What the last attempt was submitted with. Deliberately *not* cleared when
   * the progress screen is dismissed: the form remounts on the way back, and
   * this is what it has to start from if the user's own edits are to survive a
   * failed create. `creationStarted` — not this — decides which screen shows.
   */
  const [submittedValues, setSubmittedValues] = useState<GeneralApiCreationFormState | null>(null);
  /** Whether the progress screen stands in for the form. */
  const [creationStarted, setCreationStarted] = useState(false);
  /**
   * Why the last attempt was rejected, when the form is where it belongs.
   * Cleared on the next submission, not on the way back — the form is what
   * renders it, and it has to survive being returned to.
   */
  const [formErrors, setFormErrors] = useState<CreateApiFormErrors | null>(null);

  const createApi = (values: GeneralApiCreationFormState) => {
    const projectId = activeScope.projectHandler;
    if (!projectId) {
      // Nothing to create against — the wizard is mounted outside a project.
      return;
    }

    setFormErrors(null);
    // Clears the previous attempt's error before the next one starts: the
    // progress screen reads its status from this mutation, and a stale
    // `isError` would show it as failed for the frame before the retry
    // registers as pending.
    createRestApiMutation.reset();
    createRestApiMutation.mutate(toCreateRestApiBody(values, { projectId }), {
      onError: (error) => {
        // A rejection the user can fix by editing goes straight back to the
        // form with the reason attached. Standing on a screen that says only
        // "we could not create this" would hide the one thing they need —
        // which value to change — behind a second click.
        const rejection = toCreateApiFormErrors(error);
        if (!rejection) return; // Not the form's to fix: the progress screen keeps it.

        setFormErrors(rejection);
        setCreationStarted(false);
      },
    });
  };

  const onGeneralFormSumit = (finalData: GeneralApiCreationFormState) => {
    if (!activeScope.projectHandler) return;

    setSubmittedValues(finalData);
    setCreationStarted(true);
    createApi(finalData);
  };

  /**
   * Redirect to the created API's overview page.
   * `replace` keeps Back from returning to a finished progress screen.
   */
  const goToCreatedApi = useCallback(() => {
    const { orgHandle, projectHandler } = params;
    if (!orgHandle || !projectHandler) return;

    const createdId = createRestApiMutation.data?.id;
    navigate(
      createdId
        ? routes.api(orgHandle, projectHandler, createdId)
        : // Created, but the response carried no handle to navigate to.
          routes.apis(orgHandle, projectHandler),
      { replace: true },
    );
  }, [createRestApiMutation.data?.id, navigate, params]);

  const creationStatus: ApiCreationProgressStatus = createRestApiMutation.isError
    ? 'failed'
    : createRestApiMutation.isSuccess
      ? 'created'
      : 'creating';

  if (creationStarted && submittedValues) {
    return (
      <ApiCreationProgress
        displayName={submittedValues.displayName}
        onBack={() => {
          createRestApiMutation.reset();
          // Only the screen goes back; `submittedValues` stays so the form
          // returns to what was typed rather than to the imported draft.
          setCreationStarted(false);
        }}
        onComplete={goToCreatedApi}
        onRetry={() => createApi(submittedValues)}
        status={creationStatus}
      />
    );
  }

  return (
    <Box sx={{ width: '100%' }}>
      <Stack direction="column" spacing={2} sx={{ alignItems: 'center' }}>
        <ApiCreationSteps activeStep={step} onStepClick={(step) => setStep(step)} />

        <Box sx={{ alignSelf: 'flex-start' }}>
          <Typography variant="h1" sx={{ textAlign: 'left', mb: 1, fontWeight: 700 }}>
            {getTitleForStep(step)}
          </Typography>
          <Typography variant="body1" sx={{ textAlign: 'left' }}>
            {getSubtitleForStep(step)}
          </Typography>
        </Box>

        <Box sx={{ alignSelf: 'flex-start', width: '100%' }}>
          {step === 'apiType' && (
            <ApiTypeSelector
              onChange={(apiType) => {
                setApiType(apiType);
                setStep('source');
              }}
            />
          )}

          {step === 'source' && (
            // `onAuthorizeGitHub` and `onRefreshSwaggerHubOrganizations` are
            // deliberately not passed: neither flow is wired yet, and the
            // panel hides the control belonging to a handler it wasn't given
            // rather than rendering a button that does nothing.
            <DefineApiPanel
              initialApiTypeKey={apiType?.key}
              onDataFetched={handleDataExtracted}
              onBack={() => setStep('apiType')}
            />
          )}

          {step === 'configure' && (
            <Box sx={{ maxWidth: '80%', mt: 1 }}>
              <GeneralCreateApiForm
                // What the user actually submitted, when there is such an
                // attempt to come back from: the form remounts after the
                // progress screen, so anything hand-typed would otherwise
                // revert to the spec-derived draft.
                initialValues={submittedValues ?? prefilledData}
                onSubmit={onGeneralFormSumit}
                onBack={() => setStep('source')}
                serverErrors={formErrors ?? undefined}
              />
            </Box>
          )}
        </Box>
      </Stack>
    </Box>
  );
};
