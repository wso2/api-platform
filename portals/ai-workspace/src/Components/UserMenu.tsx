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

import React, { useState } from 'react';
import {
  Avatar,
  Box,
  Chip,
  Divider,
  IconButton,
  ListItemIcon,
  ListItemText,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
  styled,
} from '@wso2/oxygen-ui';
import {
  ChevronRight,
  CreditCard,
  LogOut,
  Settings,
  User as UserIcon,
} from '@wso2/oxygen-ui-icons-react';

const UserMenuTrigger = styled(IconButton, {
  name: 'MuiUserMenu',
  slot: 'Trigger',
})(({ theme }) => ({
  padding: theme.spacing(0.5),
}));

const UserMenuAvatar = styled(Avatar, {
  name: 'MuiUserMenu',
  slot: 'Avatar',
})(({ theme }) => ({
  width: 32,
  height: 32,
  backgroundColor: (theme.vars || theme).palette.primary.main,
  color: (theme.vars || theme).palette.primary.contrastText,
  fontSize: 14,
  fontWeight: 600,
}));

const UserMenuHeaderAvatar = styled(Avatar, {
  name: 'MuiUserMenu',
  slot: 'HeaderAvatar',
})(({ theme }) => ({
  width: 40,
  height: 40,
  backgroundColor: (theme.vars || theme).palette.primary.main,
  color: (theme.vars || theme).palette.primary.contrastText,
  fontSize: 16,
  fontWeight: 600,
}));

const UserMenuHeader = styled(Box, {
  name: 'MuiUserMenu',
  slot: 'Header',
})(({ theme }) => ({
  paddingLeft: theme.spacing(2),
  paddingRight: theme.spacing(2),
  paddingTop: theme.spacing(1.5),
  paddingBottom: theme.spacing(1.5),
}));

const UserMenuHeaderContent = styled(Box, {
  name: 'MuiUserMenu',
  slot: 'HeaderContent',
})(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1.5),
}));

const UserMenuUserInfo = styled(Box, {
  name: 'MuiUserMenu',
  slot: 'UserInfo',
})({
  flex: 1,
  minWidth: 0,
});

const UserMenuNameRow = styled(Box, {
  name: 'MuiUserMenu',
  slot: 'NameRow',
})(({ theme }) => ({
  display: 'flex',
  alignItems: 'center',
  gap: theme.spacing(1),
}));

const UserMenuName = styled(Typography, {
  name: 'MuiUserMenu',
  slot: 'Name',
})({
  fontWeight: 600,
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
});

const UserMenuRoleChip = styled(Chip, {
  name: 'MuiUserMenu',
  slot: 'RoleChip',
})({
  height: 20,
  fontSize: 11,
  fontWeight: 600,
});

const UserMenuEmail = styled(Typography, {
  name: 'MuiUserMenu',
  slot: 'Email',
})(({ theme }) => ({
  color: (theme.vars || theme).palette.text.secondary,
  display: 'block',
  whiteSpace: 'nowrap',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
}));

const UserMenuMenuItem = styled(MenuItem, {
  name: 'MuiUserMenu',
  slot: 'MenuItem',
})(({ theme }) => ({
  paddingTop: theme.spacing(1.5),
  paddingBottom: theme.spacing(1.5),
}));

const UserMenuLogoutItem = styled(MenuItem, {
  name: 'MuiUserMenu',
  slot: 'LogoutItem',
})(({ theme }) => ({
  paddingTop: theme.spacing(1.5),
  paddingBottom: theme.spacing(1.5),
  color: (theme.vars || theme).palette.error.main,
  '&:hover': {
    backgroundColor: (theme.vars || theme).palette.error.main,
    color: (theme.vars || theme).palette.error.contrastText,
    '& .MuiListItemIcon-root': {
      color: (theme.vars || theme).palette.error.contrastText,
    },
  },
}));

const UserMenuLogoutIcon = styled(ListItemIcon, {
  name: 'MuiUserMenu',
  slot: 'LogoutIcon',
})(({ theme }) => ({
  color: (theme.vars || theme).palette.error.main,
}));

const UserMenuBillingChip = styled(Chip, {
  name: 'MuiUserMenu',
  slot: 'BillingChip',
})({
  height: 18,
  fontSize: 10,
});

export interface UserMenuUser {
  name: string;
  email: string;
  role?: string;
}

export interface UserMenuProps {
  user: UserMenuUser;
  onProfileClick?: () => void;
  onSettingsClick?: () => void;
  onBillingClick?: () => void;
  onLogout?: () => void;
}

export default function UserMenu({
  user,
  onProfileClick,
  onSettingsClick,
  onBillingClick,
  onLogout,
}: UserMenuProps) {
  const [profilePicUrl] = useState<string | undefined>(undefined);

  const [anchorEl, setAnchorEl] = React.useState<null | HTMLElement>(null);
  const open = Boolean(anchorEl);
  const hasNavItems = Boolean(
    onProfileClick || onSettingsClick || onBillingClick
  );

  const handleClick = (event: React.MouseEvent<HTMLButtonElement>) => {
    setAnchorEl(event.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleMenuAction = (callback?: () => void) => {
    handleClose();
    callback?.();
  };

  return (
    <>
      <Tooltip title="Account">
        <UserMenuTrigger
          onClick={handleClick}
          size="small"
          aria-controls={open ? 'user-menu' : undefined}
          aria-haspopup="true"
          aria-expanded={open ? 'true' : undefined}
        >
          <UserMenuAvatar src={profilePicUrl || undefined} alt={user.name}>
            {!profilePicUrl && (user.name || 'U').charAt(0).toUpperCase()}
          </UserMenuAvatar>
        </UserMenuTrigger>
      </Tooltip>

      <Menu
        id="user-menu"
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        slotProps={{
          paper: {
            sx: {
              minWidth: 240,
              mt: 1,
            },
          },
        }}
      >
        <UserMenuHeader>
          <UserMenuHeaderContent>
            <UserMenuHeaderAvatar src={profilePicUrl || undefined} alt={user.name}>
              {!profilePicUrl && (user.name || 'U').charAt(0).toUpperCase()}
            </UserMenuHeaderAvatar>
            <UserMenuUserInfo>
              <UserMenuNameRow>
                <UserMenuName variant="subtitle2">{user.name}</UserMenuName>
              </UserMenuNameRow>
              <UserMenuEmail variant="caption">{user.email}</UserMenuEmail>
            </UserMenuUserInfo>
          </UserMenuHeaderContent>
        </UserMenuHeader>

        <Divider />

        {onProfileClick && (
          <UserMenuMenuItem onClick={() => handleMenuAction(onProfileClick)}>
            <ListItemIcon>
              <UserIcon size={18} />
            </ListItemIcon>
            <ListItemText primary="Profile" />
            <ChevronRight
              size={16}
              style={{ color: 'var(--mui-palette-text-secondary)' }}
            />
          </UserMenuMenuItem>
        )}

        {onSettingsClick && (
          <UserMenuMenuItem onClick={() => handleMenuAction(onSettingsClick)}>
            <ListItemIcon>
              <Settings size={18} />
            </ListItemIcon>
            <ListItemText primary="Settings" />
            <ChevronRight
              size={16}
              style={{ color: 'var(--mui-palette-text-secondary)' }}
            />
          </UserMenuMenuItem>
        )}

        {onBillingClick && (
          <UserMenuMenuItem onClick={() => handleMenuAction(onBillingClick)}>
            <ListItemIcon>
              <CreditCard size={18} />
            </ListItemIcon>
            <ListItemText primary="Billing" />
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
              {user.role && (
                <UserMenuBillingChip
                  label={user.role}
                  size="small"
                  variant="outlined"
                />
              )}
              <ChevronRight
                size={16}
                style={{ color: 'var(--mui-palette-text-secondary)' }}
              />
            </Box>
          </UserMenuMenuItem>
        )}

        {hasNavItems && <Divider />}

        <UserMenuLogoutItem onClick={() => handleMenuAction(onLogout)}>
          <UserMenuLogoutIcon>
            <LogOut size={18} />
          </UserMenuLogoutIcon>
          <ListItemText primary="Log out" />
        </UserMenuLogoutItem>
      </Menu>
    </>
  );
}
