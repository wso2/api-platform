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

import { createContext, ReactNode, useCallback, useContext, useMemo, useState } from 'react';
import { Alert, Button, Snackbar } from '@wso2/oxygen-ui';

type NotificationSeverity = 'success' | 'info' | 'warning' | 'error';

/**
 * One follow-up a notification can offer, such as "Deploy it" after an API is
 * created. A callback rather than a route: the provider sits above the router,
 * so the caller, which has `navigate`, decides what the action does.
 */
export type NotificationAction = {
  label: string;
  onClick: () => void;
};

type Notification = {
  action?: NotificationAction;
  message: string;
  severity: NotificationSeverity;
};

type NotificationContextValue = {
  notify: (message: string, severity?: NotificationSeverity, action?: NotificationAction) => void;
};

const NotificationContext = createContext<NotificationContextValue | null>(null);

/** How long a notification stays up; one with a next step gets longer to act on. */
const AUTO_HIDE_MS = 5000;
const AUTO_HIDE_WITH_ACTION_MS = 10000;

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [notification, setNotification] = useState<Notification | null>(null);

  const notify = useCallback(
    (message: string, severity: NotificationSeverity = 'info', action?: NotificationAction) => {
      setNotification({ action, message, severity });
    },
    [],
  );

  const value = useMemo(() => ({ notify }), [notify]);
  const action = notification?.action;

  return (
    <NotificationContext.Provider value={value}>
      {children}
      <Snackbar
        anchorOrigin={{ horizontal: 'right', vertical: 'bottom' }}
        autoHideDuration={action ? AUTO_HIDE_WITH_ACTION_MS : AUTO_HIDE_MS}
        open={!!notification}
        onClose={() => setNotification(null)}
      >
        {notification ? (
          <Alert
            action={
              action ? (
                <Button
                  color="inherit"
                  onClick={() => {
                    setNotification(null);
                    action.onClick();
                  }}
                  size="small"
                >
                  {action.label}
                </Button>
              ) : undefined
            }
            severity={notification.severity}
          >
            {notification.message}
          </Alert>
        ) : undefined}
      </Snackbar>
    </NotificationContext.Provider>
  );
}

export const useNotifications = () => {
  const context = useContext(NotificationContext);
  if (!context) {
    throw new Error('useNotifications must be used within NotificationProvider');
  }
  return context;
};
