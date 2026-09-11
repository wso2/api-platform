/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { type FC } from 'react';
import {
  Box,
  List,
  ListItemButton,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';

import type { Environment } from '../types';

export type EnvironmentPickerProps = {
  open: boolean;
  anchorEl: HTMLElement | null;
  /** All environments the pipeline could target. */
  environments: Environment[];
  /** Environment names already in this pipeline — offered but disabled, not hidden, so the count stays legible. One pipeline may use a given environment at most once. */
  usedEnvironments: string[];
  onClose: () => void;
  /** Called with the chosen environment's name (the API's identifier for it). */
  onAdd: (environmentName: string) => void;
};

/**
 * The environment picker. A pipeline is a promotion order over environments and
 * nothing else — which gateway an environment deploys to is settled on the
 * gateway itself, when it is onboarded — so this is a single-step list with no
 * gateway involved.
 */
const EnvironmentPicker: FC<EnvironmentPickerProps> = ({
  open,
  anchorEl,
  environments,
  usedEnvironments,
  onClose,
  onAdd,
}) => {
  const isEnvironmentUsed = (name: string) => usedEnvironments.includes(name);

  const handleSelectEnvironment = (environment: Environment) => {
    onAdd(environment.name);
    onClose();
  };

  return (
    <Popover
      open={open}
      anchorEl={anchorEl}
      onClose={onClose}
      anchorOrigin={{ vertical: 'bottom', horizontal: 'left' }}
      transformOrigin={{ vertical: 'top', horizontal: 'left' }}
    >
      <Box sx={{ width: 260 }}>
        <Typography variant="subtitle2" sx={{ px: 2, pt: 1.5, pb: 0.5 }}>
          Select Environment
        </Typography>
        <List dense sx={{ py: 0.5 }}>
          {environments.map((environment) => {
            const used = isEnvironmentUsed(environment.name);
            return (
              <Tooltip
                key={environment.name}
                title={used ? 'This environment is already part of the pipeline.' : ''}
                placement="right"
              >
                <Box component="span" sx={{ display: 'block' }}>
                  <ListItemButton
                    disabled={used}
                    onClick={() => handleSelectEnvironment(environment)}
                  >
                    <ListItemText
                      primary={environment.name}
                      secondary={
                        used ? 'Already added' : environment.critical ? 'Critical' : undefined
                      }
                    />
                  </ListItemButton>
                </Box>
              </Tooltip>
            );
          })}
          {/* An empty list is checked first: `every` is true of no elements, so an
              organization with no environments at all would otherwise be told they
              had all been added — the opposite of what it needs to do next. */}
          {environments.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
              No environments yet. Create one before building a pipeline.
            </Typography>
          ) : environments.every((environment) => isEnvironmentUsed(environment.name)) ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
              All environments have been added.
            </Typography>
          ) : null}
        </List>
      </Box>
    </Popover>
  );
};

export default EnvironmentPicker;
