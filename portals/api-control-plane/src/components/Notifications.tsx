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
