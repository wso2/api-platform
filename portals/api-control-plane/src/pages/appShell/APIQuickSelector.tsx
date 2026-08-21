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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Box,
  Divider,
  IconButton,
  InputAdornment,
  Menu,
  MenuItem,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronRight, Search } from '@wso2/oxygen-ui-icons-react';
import { FormattedMessage, useIntl } from 'react-intl';

type SelectableApi = {
  id: string;
  name: string;
  description?: string;
};

type Props = {
  disabled: boolean;
  isApisLoading: boolean;
  apisError?: unknown;
  apiOptions: SelectableApi[];
  onSelectApi: (apiId: string) => void;
};

export default function APIQuickSelector({
  disabled,
  isApisLoading,
  apisError,
  apiOptions,
  onSelectApi,
}: Props) {
  const intl = useIntl();
  const [anchorEl, setAnchorEl] = useState<null | HTMLElement>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const open = Boolean(anchorEl);

  useEffect(() => {
    if (!disabled) return;
    setAnchorEl(null);
    setSearchQuery('');
  }, [disabled]);

  const filteredApis = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) return apiOptions;
    return apiOptions.filter((api) => {
      const haystack = [api.name, api.description]
        .filter(Boolean)
        .join(' ')
        .toLowerCase();
      return haystack.includes(query);
    });
  }, [apiOptions, searchQuery]);

  const handleClose = () => {
    setAnchorEl(null);
    setSearchQuery('');
  };

  return (
    <>
      <Tooltip title={intl.formatMessage({ id: 'appShell.header.api.select', defaultMessage: 'Select API' })}>
        <span>
          <IconButton
            size="small"
            aria-label={intl.formatMessage({ id: 'appShell.header.api.select', defaultMessage: 'Select API' })}
            disabled={disabled}
            onClick={(event) => setAnchorEl(event.currentTarget)}
            sx={{
              width: 32,
              height: 32,
              border: '1px solid',
              borderColor: 'divider',
              // borderRadius: 1,
            }}
          >
            <ChevronRight size={16} />
          </IconButton>
        </span>
      </Tooltip>

      <Menu
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
        transformOrigin={{ vertical: 'top', horizontal: 'left' }}
        slotProps={{
          paper: {
            sx: {
              mt: 1,
              width: 280,
              borderRadius: 1,
            },
          },
        }}
      >
        <Box sx={{ px: 1.5, pt: 1.2, pb: 1 }}>
          <Typography
            variant="caption"
            sx={{ color: 'primary.main', display: 'block', mb: 1 }}
          >
            <FormattedMessage
              id="aiWorkspace.pages.appShell.APIQuickSelector.apis"
              defaultMessage="APIs"
            />
          </Typography>
          <TextField
            size="small"
            fullWidth
            autoFocus
            placeholder={intl.formatMessage({ id: 'appShell.header.api.search', defaultMessage: 'Search' })}
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            slotProps={{
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <Search size={16} />
                  </InputAdornment>
                ),
              },
            }}
          />
        </Box>
        <Divider />
        <Box sx={{ maxHeight: 260, overflowY: 'auto', pb: 0.5 }}>
          {isApisLoading ? (
            <MenuItem disabled>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.APIQuickSelector.loading"
                defaultMessage='Loading...'
              />
            </MenuItem>
          ) : apisError ? (
            <MenuItem disabled>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.APIQuickSelector.failed.to.load.apis"
                defaultMessage='Failed to load APIs'
              />
            </MenuItem>
          ) : 
          (filteredApis.length === 0 ? (
            <MenuItem disabled>
              <FormattedMessage
                id="aiWorkspace.pages.appShell.APIQuickSelector.no.apis.available"
                defaultMessage='No APIs available'
              />
            </MenuItem>
          ) : (
            filteredApis.map(api => (
                <MenuItem
                key={api.id}
                onClick={() => {
                  handleClose();
                  onSelectApi(api.id);
                }}
              >
                {api.name}
              </MenuItem>
            ))
          ))}
        </Box>
      </Menu>
    </>
  );
}

