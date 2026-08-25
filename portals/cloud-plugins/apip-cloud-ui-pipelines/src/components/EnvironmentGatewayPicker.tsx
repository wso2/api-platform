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

import type { Environment, PipelineStage } from '../types';

export type EnvironmentGatewayPickerProps = {
  open: boolean;
  anchorEl: HTMLElement | null;
  /** All environments the pipeline could target. */
  environments: Environment[];
  /** Stages already in this pipeline — an environment used by any of them is offered but disabled, not hidden, so the count stays legible. One pipeline may use a given environment at most once. */
  usedStages: Pick<PipelineStage, 'environmentId'>[];
  onClose: () => void;
  /**
   * Called once the user confirms adding an environment — the whole
   * environment (every gateway it has) becomes the stage; `defaultGatewayId`
   * is whichever gateway they toggled on. An environment with exactly one
   * gateway skips straight to this call, that gateway as the default — no
   * toggle step to show.
   */
  onAdd: (environmentId: string, defaultGatewayId: string) => void;
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
  usedStages,
  onClose,
  onAdd,
}) => {
  const [selectedEnvironmentId, setSelectedEnvironmentId] = useState<string | null>(null);
  const [defaultGatewayId, setDefaultGatewayId] = useState<string | null>(null);

  useEffect(() => {
    if (!open) {
      setSelectedEnvironmentId(null);
      setDefaultGatewayId(null);
    }
  }, [open]);

  const selectedEnvironment = environments.find((env) => env.id === selectedEnvironmentId) ?? null;

  const isEnvironmentUsed = (environmentId: string) =>
    usedStages.some((stage) => stage.environmentId === environmentId);

  const handleSelectEnvironment = (environment: Environment) => {
    if (environment.gateways.length <= 1) {
      const [onlyGateway] = environment.gateways;
      if (onlyGateway) onAdd(environment.id, onlyGateway.id);
      onClose();
      return;
    }
    setSelectedEnvironmentId(environment.id);
    setDefaultGatewayId(environment.gateways[0]?.id ?? null);
  };

  const handleConfirmAdd = () => {
    if (!selectedEnvironment || !defaultGatewayId) return;
    onAdd(selectedEnvironment.id, defaultGatewayId);
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
                onClick={() => setSelectedEnvironmentId(null)}
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
                const disabled = isEnvironmentUsed(environment.id);
                return (
                  <Tooltip
                    key={environment.id}
                    title={disabled ? 'This environment is already part of the pipeline.' : ''}
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
                            disabled
                              ? 'Already added'
                              : `${environment.gateways.length} gateway${environment.gateways.length === 1 ? '' : 's'}`
                          }
                        />
                      </ListItemButton>
                    </Box>
                  </Tooltip>
                );
              })}
              {environments.every((environment) => isEnvironmentUsed(environment.id)) ? (
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
