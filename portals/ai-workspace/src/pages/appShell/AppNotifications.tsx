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

import React, { useMemo, useState } from 'react';
import {
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  NotificationPanel,
  formatRelativeTime,
} from '@wso2/oxygen-ui';
import { Bell } from '@wso2/oxygen-ui-icons-react';

type Props = {
  shellState: any;
  shellActions: any;

  notifications: any[];
  unreadCount?: number;
  unreadNotifications: any[];
  notifActions: any;

  tabIndex: number;
  setTabIndex: (v: number) => void;

  onLogout: () => void;
  navigate: (to: string) => void;
};

export default function AppNotifications(props: Props) {
  const {
    shellState,
    shellActions,
    notifications,
    unreadCount,
    unreadNotifications,
    notifActions,
    tabIndex,
    setTabIndex,
    onLogout,
    navigate,
  } = props;

  const [confirmDialogOpen, setConfirmDialogOpen] = useState(false);

  const alertNotifications = useMemo(
    () =>
      notifications.filter(
        (n: any) => n.type === 'warning' || n.type === 'error'
      ),
    [notifications]
  );

  const filtered = useMemo(() => {
    switch (tabIndex) {
      case 1:
        return unreadNotifications;
      case 2:
        return alertNotifications;
      default:
        return notifications;
    }
  }, [tabIndex, notifications, unreadNotifications, alertNotifications]);

  return (
    <>
      <NotificationPanel
        open={shellState.notificationPanelOpen}
        onClose={shellActions.toggleNotificationPanel}
      >
        <NotificationPanel.Header>
          <NotificationPanel.HeaderIcon>
            <Bell size={20} />
          </NotificationPanel.HeaderIcon>
          <NotificationPanel.HeaderTitle>
            Notifications
          </NotificationPanel.HeaderTitle>
          {(unreadCount ?? 0) > 0 && (
            <NotificationPanel.HeaderBadge>
              {unreadCount}
            </NotificationPanel.HeaderBadge>
          )}
          <NotificationPanel.HeaderClose />
        </NotificationPanel.Header>

        <NotificationPanel.Tabs
          tabs={[
            { label: 'All', count: notifications.length },
            {
              label: 'Unread',
              count: unreadNotifications.length,
              color: 'primary',
            },
            {
              label: 'Alerts',
              count: alertNotifications.length,
              color: 'warning',
            },
          ]}
          value={tabIndex}
          onChange={setTabIndex}
        />

        {notifications.length > 0 && (
          <NotificationPanel.Actions
            hasUnread={unreadNotifications.length > 0}
            onMarkAllRead={notifActions.markAllRead}
            onClearAll={notifActions.clearAll}
          />
        )}

        {filtered.length === 0 ? (
          <NotificationPanel.EmptyState />
        ) : (
          <NotificationPanel.List>
            {filtered.map((notification: any) => (
              <NotificationPanel.Item
                key={notification.id}
                id={notification.id}
                type={notification.type ?? 'info'}
                read={notification.read}
                onMarkRead={notifActions.markRead}
                onDismiss={notifActions.dismiss}
              >
                <NotificationPanel.ItemAvatar>
                  {notification.avatar}
                </NotificationPanel.ItemAvatar>
                <NotificationPanel.ItemTitle>
                  {notification.title}
                </NotificationPanel.ItemTitle>
                <NotificationPanel.ItemMessage>
                  {notification.message}
                </NotificationPanel.ItemMessage>
                <NotificationPanel.ItemTimestamp>
                  {formatRelativeTime(notification.timestamp)}
                </NotificationPanel.ItemTimestamp>
                {notification.actionLabel && (
                  <NotificationPanel.ItemAction>
                    {notification.actionLabel}
                  </NotificationPanel.ItemAction>
                )}
              </NotificationPanel.Item>
            ))}
          </NotificationPanel.List>
        )}
      </NotificationPanel>

      {/* Keep Sign Out confirm here (same as your old code) */}
      <Dialog
        open={confirmDialogOpen}
        onClose={() => setConfirmDialogOpen(false)}
        maxWidth="sm"
        fullWidth
      >
        <DialogTitle>Sign Out</DialogTitle>
        <DialogContent>
          <DialogContentText>
            Are you sure you want to sign out of your account?
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            variant="outlined"
            color="secondary"
            onClick={() => setConfirmDialogOpen(false)}
          >
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={() => {
              setConfirmDialogOpen(false);
              onLogout();
              navigate('/login');
            }}
          >
            Sign Out
          </Button>
        </DialogActions>
      </Dialog>

      {/* 🔥 Important: You need a way to open this dialog from UserMenu.
          Easiest: pass a prop callback from AppLayout to AppHeader.
          If you want, I’ll show the cleanest wiring in the next message. */}
    </>
  );
}
