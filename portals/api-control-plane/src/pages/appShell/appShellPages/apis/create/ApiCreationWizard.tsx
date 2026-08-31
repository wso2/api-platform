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
import { useNavigate } from 'react-router-dom';
import { DefineApiPanel } from './components/DefineApiPanel';
import { GeneralCreateApiForm } from './components/GeneralCreateApiForm';
import { ApiCreationWizardDraftState, ApiType, GeneralApiCreationFormState } from './types';
import { ApiTypeSelector } from './components/ApiTypeSelector';
import { ApiCreationStepKey, ApiCreationSteps } from './components/ApiCreationSteps';
import { API_TYPES } from './uiConfig';
import { useCreateRestApi } from '@/api/resources/restApis';
import { useConsoleScope } from '@/scope/ConsoleScopeProvider';
import { routes } from '@/routes/paths';
import { toCreateRestApiBody } from './utils/createRestApiBody';
import {
  ApiCreationProgress,
  type ApiCreationProgressStatus,
} from './components/ApiCreationProgress';

export const ApiCreationWizard = () => {
  const [step, setStep] = useState<ApiCreationStepKey>('apiType');
  const [apiType, setApiType] = useState<ApiType | null>(null);

  const [prefilledData, setPrefilledData] = useState<Partial<GeneralApiCreationFormState>>({});

  const getTitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return 'What kind of API are you exposing?';
      case 'source':
        return `How do you want to define your ${API_TYPES.find((type) => type === apiType)?.rawTitle || 'API'}?`;
      case 'configure':
        return 'Configure and create';
      default:
        return '';
    }
  };

  const getSubtitleForStep = (step: ApiCreationStepKey) => {
    switch (step) {
      case 'apiType':
        return 'This decides how the gateway exposes your backend. Only REST is available today.';
      case 'source':
        return 'Bring an existing contract, or start from a blank slate and fill in the details yourself.';
      case 'configure':
        return `${API_TYPES.find((type) => type === apiType)?.rawTitle || 'API'}.DEFAULT SKELETON - EDIT IT OR ASK AI TO REFINE IT`;
      default:
        return '';
    }
  };

  const handleDataExtracted = (data: ApiCreationWizardDraftState) => {
    setPrefilledData((prev) => ({
      ...prev,
      ...data,
    }));

    setStep('configure');
  };

  const navigate = useNavigate();
  const createRestApiMutation = useCreateRestApi();
  // `projectId` on the request body is the project handle from the route, not
  // something the form collects.
  const { activeScope, params } = useConsoleScope();

  /**
   * The submitted values are kept while the request is in flight.
   * They also trigger the progress screen, so failed attempts can retry
   * without sending the user back through the form.
   */
  const [submittedValues, setSubmittedValues] = useState<GeneralApiCreationFormState | null>(null);

  const createApi = (values: GeneralApiCreationFormState) => {
    const projectId = activeScope.projectHandler;
    if (!projectId) {
      // Nothing to create against — the wizard is mounted outside a project.
      return;
    }

    createRestApiMutation.mutate(toCreateRestApiBody(values, { projectId }));
  };

  const onGeneralFormSumit = (finalData: GeneralApiCreationFormState) => {
    if (!activeScope.projectHandler) return;

    setSubmittedValues(finalData);
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

  if (submittedValues) {
    return (
      <ApiCreationProgress
        displayName={submittedValues.displayName}
        onBack={() => {
          createRestApiMutation.reset();
          setSubmittedValues(null);
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
            <DefineApiPanel
              onAuthorizeGitHub={() => {}}
              onDataFetched={handleDataExtracted}
              onBack={() => setStep('apiType')}
              onRefreshSwaggerHubOrganizations={() => {}}
            />
          )}

          {step === 'configure' && (
            <Box sx={{ maxWidth: '80%', mt: 1 }}>
              <GeneralCreateApiForm
                initialValues={prefilledData}
                onSubmit={onGeneralFormSumit}
                onBack={() => setStep('source')}
              />
            </Box>
          )}
        </Box>
      </Stack>
    </Box>
  );
};
