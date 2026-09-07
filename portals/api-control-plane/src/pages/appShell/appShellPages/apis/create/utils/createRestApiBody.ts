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

import type { CreateRestApiBody } from '@/api/resources/restApis';
import type { ApiOperation, GeneralApiCreationFormState, UpStreamTarget } from '../types';

/**
 * Maps the wizard's form state onto the `CreateRESTAPI` request body.
 *
 * The two shapes are close but not the same, and the differences are the whole
 * reason this module exists:
 *
 * - **`projectId` isn't in the form.** It comes from route scope, so the caller
 *   passes it in — the form has no business knowing which project it sits in.
 * - **`transports` vs `transport`.** The spec field is singular.
 * - **`kind` is `RestApi`.** The form state's literal says `RestApis`, but the
 *   spec's default/example — and the enum every other artifact reference uses
 *   (`RestApi | LlmProvider | LlmProxy | Mcp`) — is the singular form. Emitting
 *   the form's value verbatim would send a kind the platform doesn't know.
 * - **Server-owned fields never travel.** `readOnly`, `createdBy`, `createdAt`
 *   and friends are `readonly` in the spec; the body is built up field by field
 *   rather than spread from the form, so they cannot leak in by accident.
 * - **Blank optionals are dropped, not sent empty.** An untouched Description or
 *   Sandbox URL is *absent* from the body. `""` would otherwise be stored as a
 *   real value, and an empty `upstream.sandbox` would fail the spec's
 *   "exactly one of url/ref" rule on that target.
 *
 * Pure and side-effect free: no scope reads, no notifications, no request. Feed
 * the result straight to `useCreateRestApi().mutate`.
 */

/** What the platform calls a REST API artifact. Not the form's `kind` literal. */
const REST_API_KIND = 'RestApi';

type UpstreamDefinition = CreateRestApiBody['upstream']['main'];
type RequestOperation = NonNullable<CreateRestApiBody['operations']>[number];

/** A trimmed value, or `undefined` when there is nothing left to send. */
const trimmed = (value: string | undefined): string | undefined => {
  const next = value?.trim();
  return next === '' ? undefined : next;
};

/**
 * Routing context as the gateway expects it: a single leading slash. The form
 * lets the user type `orders` or `/orders`; both mean the same base path.
 */
const toContext = (value: string): string => {
  const next = value.trim().replace(/^\/+/, '');
  return `/${next}`;
};

/**
 * One upstream target. `url` and `ref` are mutually exclusive in the spec, so a
 * target carrying both sends only `ref`, the more specific of the two.
 *
 * With no `ref`, `url` is sent even when blank rather than dropped: the target
 * has to carry one of the two, and an empty string comes back as a field error
 * naming `upstream.main.url`, where a missing key reads as a malformed body.
 */
const toUpstreamDefinition = (target: UpStreamTarget): UpstreamDefinition => {
  const ref = trimmed(target.ref);
  const authType = target.auth?.type;

  return {
    ...(ref === undefined ? { url: target.url.trim() } : { ref }),
    ...(authType === undefined
      ? {}
      : {
          auth: {
            type: authType,
            ...(trimmed(target.auth?.header) === undefined
              ? {}
              : { header: trimmed(target.auth?.header) }),
            ...(trimmed(target.auth?.value) === undefined
              ? {}
              : { value: trimmed(target.auth?.value) }),
          },
        }),
  };
};

const toRequestOperation = (operation: ApiOperation): RequestOperation => {
  const description = trimmed(operation.description);

  return {
    name: operation.name,
    ...(description === undefined ? {} : { description }),
    request: {
      method: operation.request.method,
      path: operation.request.path,
    },
  };
};

export const toCreateRestApiBody = (
  formState: GeneralApiCreationFormState,
  scope: { projectId: string },
): CreateRestApiBody => {
  const id = trimmed(formState.id);
  const description = trimmed(formState.description);
  const { main, sandbox } = formState.upstream;
  const sandboxUrl = trimmed(sandbox?.url);

  return {
    // Omitted when blank: the platform derives the handle from the display
    // name, and sending `""` would be rejected rather than auto-filled.
    ...(id === undefined ? {} : { id }),
    displayName: formState.displayName.trim(),
    ...(description === undefined ? {} : { description }),
    context: toContext(formState.context),
    version: formState.version.trim(),
    projectId: scope.projectId,
    kind: REST_API_KIND,
    lifeCycleStatus: formState.lifeCycleStatus,
    upstream: {
      main: toUpstreamDefinition(main),
      // A sandbox with no URL is no sandbox at all.
      ...(sandbox === undefined || sandboxUrl === undefined
        ? {}
        : { sandbox: toUpstreamDefinition(sandbox) }),
    },
    ...(formState.transports.length === 0 ? {} : { transport: [...formState.transports] }),
    ...(formState.operations.length === 0
      ? {}
      : { operations: formState.operations.map(toRequestOperation) }),
  };
};
