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

import { FilePlus2, FlaskConical, Globe, Rocket } from '@wso2/oxygen-ui-icons-react';
import { useIntl } from 'react-intl';
import { useNavigate, useParams } from 'react-router-dom';

import { routes } from '@/routes/paths';
import {
  ProgressStepper,
  progressStepMessages,
  type ProgressStep,
} from '../../apis/overview/ProgressBanner';

/**
 * The REST overview's progress stepper for a GraphQL API — same rendering
 * (`ProgressStepper`), GraphQL's own completion signals. Only two steps track
 * real completion (Create is always done; Deploy reflects whether the API is
 * live on any gateway): `GraphQLAPIDetail` carries no `lifeCycleStatus`, so
 * there is no honest signal for "tested" the way REST's stepper has. Test is
 * offered once deployed but never marked complete. Publish stays disabled: it
 * leads to a `ComingSoon` stub with nothing behind it yet.
 */
export function GraphqlProgressBanner({ deployed }: { deployed: boolean }) {
  const { orgHandle = '', projectHandler = '', graphqlApiHandler = '' } = useParams();
  const navigate = useNavigate();
  const intl = useIntl();

  const steps: ProgressStep[] = [
    {
      key: 'create',
      label: intl.formatMessage(progressStepMessages.stepCreate),
      Icon: FilePlus2,
      complete: true,
    },
    {
      key: 'deploy',
      label: intl.formatMessage(progressStepMessages.stepDeploy),
      Icon: Rocket,
      complete: deployed,
      onClick: () =>
        navigate(routes.graphqlApiDeploy(orgHandle, projectHandler, graphqlApiHandler)),
    },
    {
      key: 'test',
      label: intl.formatMessage(progressStepMessages.stepTest),
      Icon: FlaskConical,
      complete: false,
      onClick: deployed
        ? () => navigate(routes.graphqlApiTestConsole(orgHandle, projectHandler, graphqlApiHandler))
        : undefined,
    },
    {
      key: 'publish',
      label: intl.formatMessage(progressStepMessages.stepPublish),
      Icon: Globe,
      complete: false,
    },
  ];

  return <ProgressStepper steps={steps} />;
}
