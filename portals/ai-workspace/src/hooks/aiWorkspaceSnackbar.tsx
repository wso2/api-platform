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

import { ReactNode } from 'react';
import Notification, {
  NotificationProps,
} from '../Components/Notification';
import { useAIWorkspaceSnackbarContext } from '../contexts/AIWorkspaceSnackbarContext';

interface AIWorkspaceSnackbarOptions {
  closeIcon?: boolean;
  autoHideDuration?: number;
  color?: NotificationProps['color'];
}

export const useAIWorkspaceSnackbar = () => {
  const { enqueueSnackbar } = useAIWorkspaceSnackbarContext();

  const showSnackbar = (
    message: ReactNode,
    colorOrOptions?: NotificationProps['color'] | AIWorkspaceSnackbarOptions,
    optionsOverride?: AIWorkspaceSnackbarOptions
  ) => {
    const options =
      typeof colorOrOptions === 'string'
        ? { ...optionsOverride, color: colorOrOptions }
        : colorOrOptions ?? {};
    const color = options.color ?? 'success';
    const closeIcon = options.closeIcon ?? true;

    enqueueSnackbar(
      <Notification
        testId="aiworkspace-snackbar-notification"
        color={color}
        closeIcon={closeIcon}
        message={message}
      />,
      {
        autoHideDuration: options.autoHideDuration,
      }
    );
  };

  return showSnackbar;
};

export default useAIWorkspaceSnackbar;
