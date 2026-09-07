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

interface PartialLoadWarningProps {
  /** Short, user-facing note about which source could not be loaded. */
  message: string;
  onRetry: () => void;
  retryLabel?: string;
}

/**
 * Non-blocking notice for a list assembled from several independent sources
 * (for example, Policy Hub policies plus the organization's custom policies).
 * One source failing must not hide the rest, so the caller keeps rendering
 * whatever loaded and shows this above it with a retry for the failed source.
 */
export default function PartialLoadWarning({
  message,
  onRetry,
  retryLabel = 'Retry',
}: PartialLoadWarningProps) {
  return (
    <Alert
      severity="warning"
      sx={{ mb: 1 }}
      action={
        <Button
          color="inherit"
          size="small"
          startIcon={<RefreshCcw size={14} />}
          onClick={onRetry}
        >
          {retryLabel}
        </Button>
      }
    >
      {message}
    </Alert>
  );
}
