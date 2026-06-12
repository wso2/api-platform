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
import { RefreshCcw } from '@wso2/oxygen-ui-icons-react';

/**
 * Extract HTTP status code from various error shapes:
 * - Axios/Asgardeo errors: error.response.status
 * - Public API client errors: error.message contains "status: 4XX"
 */
function getHttpStatusCode(error?: Error | null): number | null {
  if (!error) return null;

  const axiosStatus = (error as any)?.response?.status;
  if (typeof axiosStatus === 'number') return axiosStatus;

  const match = error.message?.match(/status:\s*(\d{3})/);
  if (match) return parseInt(match[1], 10);

  return null;
}

function getErrorMessage(error?: Error | null): string {
  if (!error) return 'Something went wrong.';

  const status = getHttpStatusCode(error);

  if (status !== null && status >= 400) {
    return 'Something went wrong.';
  }

  const description = (error as any)?.response?.data?.description;
  if (description) return description;

  const dataMessage = (error as any)?.response?.data?.message;
  if (dataMessage) return dataMessage;

  return error.message || 'Something went wrong.';
}

interface ErrorAlertProps {
  error?: Error | null;
  onRetry: () => void;
}

export default function ErrorAlert({ error, onRetry }: ErrorAlertProps) {
  const status = getHttpStatusCode(error);
  const isServerOrClientError = status !== null && status >= 400;

  if (isServerOrClientError) {
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
        Something went wrong.
      </Alert>
    );
  }

  return (
    <Alert severity="error">
      {getErrorMessage(error)}
    </Alert>
  );
}
