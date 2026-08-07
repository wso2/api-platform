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

import {
  createContext,
  ReactNode,
  useCallback,
  useContext,
  useMemo,
  useState,
} from 'react';
import { Alert, Snackbar } from '@wso2/oxygen-ui';

type NotificationSeverity = 'success' | 'info' | 'warning' | 'error';

type Notification = {
  message: string;
  severity: NotificationSeverity;
};

type NotificationContextValue = {
  notify: (message: string, severity?: NotificationSeverity) => void;
};

const NotificationContext = createContext<NotificationContextValue | null>(
  null
);

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [notification, setNotification] = useState<Notification | null>(null);

  const notify = useCallback(
    (message: string, severity: NotificationSeverity = 'info') => {
      setNotification({ message, severity });
    },
    []
  );

  const value = useMemo(() => ({ notify }), [notify]);

  return (
    <NotificationContext.Provider value={value}>
      {children}
      <Snackbar
        anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        autoHideDuration={5000}
        open={!!notification}
        onClose={() => setNotification(null)}
      >
        {notification ? (
          <Alert severity={notification.severity}>{notification.message}</Alert>
        ) : undefined}
      </Snackbar>
    </NotificationContext.Provider>
  );
}

export const useNotifications = () => {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error(
      'useNotifications must be used within NotificationProvider'
    );
  }
  return context;
};
