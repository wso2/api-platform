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

import type { ReactNode } from 'react';
import { useState } from 'react';
import {
  Box,
  Button,
  Chip,
  FormControl,
  FormLabel,
  Grid,
  PageContent,
  PageTitle,
  Radio,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import { Network, Sparkles, Zap } from '@wso2/oxygen-ui-icons-react';
import { Link, useNavigate, useParams } from 'react-router-dom';

import { useCreateGateway } from '../../../../api/hooks/useMvpQueries';
import { useNotifications } from '../../../../components/Notifications';
import { routes } from '../../../../routes/paths';
import type { GatewayFunctionalityType } from '../../../../types/domain';

const NAME_PATTERN = /^[a-z0-9-]{3,64}$/;

const slugify = (value: string) =>
  value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);

type GatewayTypeOption = {
  value: GatewayFunctionalityType;
  label: string;
  description: string;
  icon: ReactNode;
  beta?: boolean;
};

const GATEWAY_TYPES: GatewayTypeOption[] = [
  {
    value: 'regular',
    label: 'Regular Gateway',
    description: 'Standard API gateway for REST APIs.',
    icon: <Network size={20} />,
  },
  {
    value: 'ai',
    label: 'AI Gateway',
    description: 'Gateway optimized for AI/LLM traffic and governance.',
    icon: <Sparkles size={20} />,
  },
  {
    value: 'event',
    label: 'Event Gateway',
    description: 'Gateway for event-driven and streaming APIs.',
    icon: <Zap size={20} />,
    beta: true,
  },
];

function GatewayTypeCard({
  option,
  selected,
  onSelect,
}: {
  option: GatewayTypeOption;
  selected: boolean;
  onSelect: () => void;
}) {
  return (
    <Box
      onClick={onSelect}
      sx={{
        borderColor: selected ? 'primary.main' : 'divider',
        borderRadius: 2,
        borderStyle: 'solid',
        borderWidth: selected ? 2 : 1,
        cursor: 'pointer',
        display: 'flex',
        gap: 1,
        p: 2,
        ...(selected ? {} : { '&:hover': { borderColor: 'action.active' } }),
      }}
    >
      <Radio checked={selected} size="small" sx={{ p: 0, mt: 0.25 }} />
      <Box sx={{ minWidth: 0 }}>
        <Stack alignItems="center" direction="row" spacing={1}>
          {option.icon}
          <Typography sx={{ fontWeight: 600 }}>{option.label}</Typography>
          {option.beta && <Chip color="info" label="Beta" size="small" />}
        </Stack>
        <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
          {option.description}
        </Typography>
      </Box>
    </Box>
  );
}

export function GatewayCreatePage() {
  const { orgHandle = '' } = useParams();
  const navigate = useNavigate();
  const { notify } = useNotifications();
  const createGateway = useCreateGateway();

  const [displayName, setDisplayName] = useState('');
  const [name, setName] = useState('');
  const [nameEdited, setNameEdited] = useState(false);
  const [description, setDescription] = useState('');
  const [functionalityType, setFunctionalityType] =
    useState<GatewayFunctionalityType>('regular');
  const [endpoint, setEndpoint] = useState('');

  const onDisplayNameChange = (value: string) => {
    setDisplayName(value);
    if (!nameEdited) setName(slugify(value));
  };

  const nameValid = NAME_PATTERN.test(name);
  const canSubmit =
    displayName.trim() !== '' &&
    nameValid &&
    endpoint.trim() !== '' &&
    !createGateway.isPending;

  const submit = () => {
    createGateway.mutate(
      {
        name,
        displayName,
        vhost: endpoint.trim(),
        functionalityType,
        description: description || undefined,
      },
      {
        onSuccess: (gateway) => {
          notify(`Gateway "${gateway.displayName}" provisioned.`, 'success');
          navigate(routes.gateway(orgHandle, gateway.id));
        },
        onError: (error) =>
          notify(
            error instanceof Error
              ? error.message
              : 'Failed to provision gateway',
            'error'
          ),
      }
    );
  };

  return (
    <PageContent fullWidth>
      <PageTitle>
        <Link to={routes.gateways(orgHandle)}>
          <PageTitle.BackButton>Back to gateways</PageTitle.BackButton>
        </Link>
        <PageTitle.Header>Provision a gateway</PageTitle.Header>
        <PageTitle.SubHeader>
          Register a self-hosted gateway, then connect it to the platform.
        </PageTitle.SubHeader>
      </PageTitle>

      <Box sx={{ maxWidth: 720 }}>
        <Stack spacing={3}>
          <FormControl fullWidth>
            <FormLabel>Display name</FormLabel>
            <TextField
              onChange={(event) => onDisplayNameChange(event.target.value)}
              placeholder="Production Gateway 01"
              value={displayName}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Name</FormLabel>
            <TextField
              error={name !== '' && !nameValid}
              helperText="Lowercase letters, numbers, hyphens; 3–64 chars (unique per org)."
              onChange={(event) => {
                setNameEdited(true);
                setName(event.target.value);
              }}
              placeholder="prod-gateway-01"
              value={name}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Description (optional)</FormLabel>
            <TextField
              multiline
              minRows={2}
              onChange={(event) => setDescription(event.target.value)}
              value={description}
            />
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Gateway type</FormLabel>
            <Grid container spacing={1.5} sx={{ mt: 0 }}>
              {GATEWAY_TYPES.map((option) => (
                <Grid key={option.value} size={{ xs: 12 }}>
                  <GatewayTypeCard
                    onSelect={() => setFunctionalityType(option.value)}
                    option={option}
                    selected={functionalityType === option.value}
                  />
                </Grid>
              ))}
            </Grid>
          </FormControl>

          <FormControl fullWidth>
            <FormLabel>Virtual host</FormLabel>
            <TextField
              onChange={(event) => setEndpoint(event.target.value)}
              placeholder="https://mg.example.com:8443"
              value={endpoint}
            />
          </FormControl>

          <Stack direction="row" justifyContent="flex-end" spacing={1.5}>
            <Button
              component={Link}
              to={routes.gateways(orgHandle)}
              variant="outlined"
            >
              Cancel
            </Button>
            <Button disabled={!canSubmit} onClick={submit} variant="contained">
              {createGateway.isPending ? 'Provisioning…' : 'Provision gateway'}
            </Button>
          </Stack>
        </Stack>
      </Box>
    </PageContent>
  );
}
