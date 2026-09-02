/*
 * Copyright (c) 2026, WSO2 LLC (http://www.wso2.com). All Rights Reserved.
 *
 * This software is the property of WSO2 LLC and its suppliers, if any.
 * Dissemination of any information or reproduction of any material contained
 * herein in any form is strictly forbidden, unless permitted by WSO2 expressly.
 * You may not alter or remove any copyright or other notice from copies of this content.
 */

import { useEffect, useState, type FC } from 'react';
import {
  Box,
  Button,
  IconButton,
  List,
  ListItem,
  ListItemButton,
  ListItemText,
  Popover,
  Switch,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronLeft } from '@wso2/oxygen-ui-icons-react';

import type { Environment } from '../types';

export type EnvironmentGatewayPickerProps = {
  open: boolean;
  anchorEl: HTMLElement | null;
  /** All environments the pipeline could target. */
  environments: Environment[];
  /** Environment names already in this pipeline — offered but disabled, not hidden, so the count stays legible. One pipeline may use a given environment at most once. */
  usedEnvironments: string[];
  onClose: () => void;
  /**
   * Called once the user confirms adding an environment — `defaultGatewayId` is
   * whichever gateway they toggled on. An environment with exactly one gateway
   * skips straight to this call with that gateway as the default — no toggle
   * step to show. The environment is identified by name (the API's identifier).
   */
  onAdd: (environmentName: string, defaultGatewayId: string) => void;
};

/**
 * The environment picker: step 1 always shows environments; step 2 (marking
 * the default gateway) only appears when the chosen environment has more
 * than one gateway — with exactly one, it's the default automatically and
 * step 2 is skipped.
 */
const EnvironmentGatewayPicker: FC<EnvironmentGatewayPickerProps> = ({
  open,
  anchorEl,
  environments,
  usedEnvironments,
  onClose,
  onAdd,
}) => {
  const [selectedEnvironmentName, setSelectedEnvironmentName] = useState<string | null>(null);
  const [defaultGatewayId, setDefaultGatewayId] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setSelectedEnvironmentName(null);
      setDefaultGatewayId(null);
    }
  }, [open]);

  const selectedEnvironment =
    environments.find((env) => env.name === selectedEnvironmentName) ?? null;

  const isEnvironmentUsed = (name: string) => usedEnvironments.includes(name);

  const handleSelectEnvironment = (environment: Environment) => {
    // An environment with no gateways cannot be added — it is disabled in the
    // list, but guard here too so a stray call never silently closes the picker.
    if (environment.gateways.length === 0) return;
    if (environment.gateways.length === 1) {
      onAdd(environment.name, environment.gateways[0].id);
      onClose();
      return;
    }
    setSelectedEnvironmentName(environment.name);
    setDefaultGatewayId(environment.gateways[0]?.id ?? null);
  };

  const handleConfirmAdd = () => {
    if (!selectedEnvironment || !defaultGatewayId) return;
    onAdd(selectedEnvironment.name, defaultGatewayId);
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
        {selectedEnvironment ? (
          <>
            <Box
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 0.5,
                px: 1,
                py: 1,
                borderBottom: '1px solid',
                borderColor: 'divider',
              }}
            >
              <IconButton
                size="small"
                aria-label="Back to environments"
                onClick={() => setSelectedEnvironmentName(null)}
              >
                <ChevronLeft size={18} />
              </IconButton>
              <Typography variant="subtitle2">
                Mark default gateway in {selectedEnvironment.name}
              </Typography>
            </Box>
            <List dense sx={{ py: 0.5 }}>
              {selectedEnvironment.gateways.map((gateway) => (
                <ListItem
                  key={gateway.id}
                  secondaryAction={
                    <Switch
                      size="small"
                      checked={gateway.id === defaultGatewayId}
                      onChange={() => setDefaultGatewayId(gateway.id)}
                      inputProps={{ 'aria-label': `Mark ${gateway.name} as default` }}
                    />
                  }
                >
                  <ListItemText
                    primary={gateway.name}
                    secondary={gateway.id === defaultGatewayId ? 'Default' : undefined}
                  />
                </ListItem>
              ))}
            </List>
            <Box sx={{ px: 1.5, pb: 1.5, pt: 0.5 }}>
              <Tooltip title={!defaultGatewayId ? 'Select a default gateway before adding the environment.' : ''}>
                <span style={{ display: 'block' }}>
                  <Button fullWidth variant="contained" disabled={!defaultGatewayId} onClick={handleConfirmAdd}>
                    Add Environment
                  </Button>
                </span>
              </Tooltip>
            </Box>
          </>
        ) : (
          <>
            <Typography variant="subtitle2" sx={{ px: 2, pt: 1.5, pb: 0.5 }}>
              Select Environment
            </Typography>
            <List dense sx={{ py: 0.5 }}>
              {environments.map((environment) => {
                const used = isEnvironmentUsed(environment.name);
                const noGateways = environment.gateways.length === 0;
                const disabled = used || noGateways;
                return (
                  <Tooltip
                    key={environment.name}
                    title={
                      used
                        ? 'This environment is already part of the pipeline.'
                        : noGateways
                          ? 'This environment has no gateways yet. Add a gateway to it before including it in a pipeline.'
                          : ''
                    }
                    placement="right"
                  >
                    <Box component="span" sx={{ display: 'block' }}>
                      <ListItemButton
                        disabled={disabled}
                        onClick={() => handleSelectEnvironment(environment)}
                      >
                        <ListItemText
                          primary={environment.name}
                          secondary={
                            used
                              ? 'Already added'
                              : noGateways
                                ? 'No gateways'
                                : `${environment.gateways.length} gateway${environment.gateways.length === 1 ? '' : 's'}`
                          }
                        />
                      </ListItemButton>
                    </Box>
                  </Tooltip>
                );
              })}
              {environments.every((environment) => isEnvironmentUsed(environment.name)) ? (
                <Typography variant="body2" color="text.secondary" sx={{ px: 2, py: 1 }}>
                  All environments have been added.
                </Typography>
              ) : null}
            </List>
          </>
        )}
      </Box>
    </Popover>
  );
};

export default EnvironmentGatewayPicker;
