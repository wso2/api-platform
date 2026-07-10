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

import { Alert, Button } from '@wso2/oxygen-ui';
import { LogOut, RefreshCcw } from '@wso2/oxygen-ui-icons-react';
import { forceLogoutAndRedirect } from '../../auth/logout';
import { getErrorMessage as getShortErrorMessage, getTrackingId } from '../../utils/apiError';

/**
 * Extract HTTP status code from various error shapes:
 * - Axios/Asgardeo errors: error.response.status
 * - Platform API client errors: error.status (set by choreoApiClient)
 * - Public API client errors: error.message contains "status: 4XX"
 */
function getHttpStatusCode(error?: Error | null): number | null {
  if (!error) return null;

  const axiosStatus = (error as any)?.response?.status;
  if (typeof axiosStatus === 'number') return axiosStatus;

  const directStatus = (error as any)?.status;
  if (typeof directStatus === 'number') return directStatus;

  const match = error.message?.match(/status:\s*(\d{3})/);
  if (match) return parseInt(match[1], 10);

  return null;
}

interface ErrorAlertProps {
  error?: Error | null;
  onRetry: () => void;
}

export default function ErrorAlert({ error, onRetry }: ErrorAlertProps) {
  const status = getHttpStatusCode(error);
  const isServerError = status !== null && status >= 500;
  const isClientOrServerError = status !== null && status >= 400;

  // A 401 means the session is expired/invalid — retrying re-fires the same
  // doomed request (the spinner never resolves). Offer a logout that clears the
  // session and site cache, then redirects to the login page.
  if (status === 401) {
    return (
      <Alert
        severity="error"
        action={
          <Button
            color="error"
            size="small"
            startIcon={<LogOut size={14} />}
            onClick={() => { void forceLogoutAndRedirect(); }}
          >
            Logout
          </Button>
        }
      >
        Your session has expired. Please log in again.
      </Alert>
    );
  }

  if (isClientOrServerError) {
    // 5xx: don't surface the (possibly unhelpful) backend message — show a
    // generic notice with the trackingId so the user can quote it to support.
    // 4xx: the backend message is short, user-facing, and safe to show as-is.
    const trackingId = getTrackingId(error);
    const message = isServerError
      ? 'Something went wrong.'
      : getShortErrorMessage(error, 'Something went wrong.');

    return (
      <Alert
        severity="error"
        action={
          <Button
            color="error"
            size="small"
            startIcon={<RefreshCcw size={14} />}
            onClick={onRetry}
          >
            Retry
          </Button>
        }
      >
        {message}
        {trackingId && (
          <>
            {' '}
            <em>(Reference ID: {trackingId})</em>
          </>
        )}
      </Alert>
    );
  }

  return (
    <Alert severity="error">
      {getShortErrorMessage(error)}
    </Alert>
  );
}
