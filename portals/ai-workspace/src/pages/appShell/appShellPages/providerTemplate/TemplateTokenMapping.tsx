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

import { useMemo, useState } from 'react';
import {
  Box,
  Collapse,
  FormControl,
  FormControlLabel,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { ChevronDown, ChevronUp, Info } from '@wso2/oxygen-ui-icons-react';
import { ResourceRow } from '../../../../Components/ResourceView';
import type { ResourceMapping } from '../../../../utils/types';
import {
  fromTokenConfig,
  toTokenConfig,
  TOKEN_FIELDS,
  TOKEN_LOCATIONS,
  type TokenConfig,
  type TokenFieldKey,
} from '../../../../utils/providerTemplateFields';

interface DiscoveredResource {
  method: string;
  path: string;
  summary?: string;
}

function extractResources(spec: Record<string, unknown> | null): DiscoveredResource[] {
  const paths = (spec?.paths ?? null) as Record<
    string,
    Record<string, { summary?: string; description?: string }>
  > | null;
  if (!paths || typeof paths !== 'object') return [];
  const METHODS = new Set(['get', 'post', 'put', 'patch', 'delete', 'head', 'options']);
  const out: DiscoveredResource[] = [];
  Object.keys(paths).forEach((path) => {
    const ops = paths[path];
    if (!ops || typeof ops !== 'object') return;
    Object.keys(ops).forEach((m) => {
      if (!METHODS.has(m.toLowerCase())) return;
      out.push({
        method: m.toUpperCase(),
        path,
        summary: ops[m]?.summary || ops[m]?.description || undefined,
      });
    });
  });
  out.sort((a, b) => a.path.localeCompare(b.path) || a.method.localeCompare(b.method));
  return out;
}

function TokenFieldsEditor({
  tokens,
  onChange,
}: {
  tokens: TokenConfig;
  onChange: (field: TokenFieldKey, key: 'identifier' | 'location', value: string) => void;
}) {
  return (
    <Box
      sx={{
        display: 'grid',
        gap: 2,
        gridTemplateColumns: { xs: '1fr', md: '1fr 1fr' },
      }}
    >
      {TOKEN_FIELDS.map(({ key, label }) => (
        <FormControl key={key} fullWidth>
          <FormLabel>{label}</FormLabel>
          <Stack direction="row" spacing={1} alignItems="flex-start">
            <TextField
              fullWidth
              size="small"
              value={tokens[key].identifier}
              onChange={(e) => onChange(key, 'identifier', e.target.value)}
            />
            <Select
              size="small"
              value={tokens[key].location}
              onChange={(e) => onChange(key, 'location', e.target.value as string)}
              sx={{ minWidth: 130, flexShrink: 0 }}
            >
              {TOKEN_LOCATIONS.map((o) => (
                <MenuItem key={o.value} value={o.value}>
                  {o.label}
                </MenuItem>
              ))}
            </Select>
          </Stack>
        </FormControl>
      ))}
    </Box>
  );
}

interface TemplateTokenMappingProps {
  defaultTokens: TokenConfig;
  onChangeDefaultToken: (
    field: TokenFieldKey,
    key: 'identifier' | 'location',
    value: string
  ) => void;
  resourceMappings: ResourceMapping[];
  onChangeResourceMappings: (next: ResourceMapping[]) => void;
  spec: Record<string, unknown> | null;
  hidePerResource?: boolean;
}

export default function TemplateTokenMapping({
  defaultTokens,
  onChangeDefaultToken,
  resourceMappings,
  onChangeResourceMappings,
  spec,
  hidePerResource = false,
}: TemplateTokenMappingProps) {
  const [scope, setScope] = useState<'default' | 'resource'>('default');
  const [search, setSearch] = useState('');
  const [openKey, setOpenKey] = useState<string | null>(null);

  const resources = useMemo(() => extractResources(spec), [spec]);
  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return resources;
    return resources.filter((r) =>
      `${r.method} ${r.path} ${r.summary ?? ''}`.toLowerCase().includes(q)
    );
  }, [resources, search]);

  const mappingFor = (path: string) =>
    resourceMappings.find((m) => m.resource === path);

  const isOverridden = (path: string) => Boolean(mappingFor(path));

  const toggleOverride = (path: string, enabled: boolean) => {
    if (enabled) {
      if (isOverridden(path)) return;
      const seeded: ResourceMapping = {
        resource: path,
        ...fromTokenConfig(defaultTokens),
      };
      onChangeResourceMappings([...resourceMappings, seeded]);
      setOpenKey(path);
    } else {
      onChangeResourceMappings(resourceMappings.filter((m) => m.resource !== path));
    }
  };

  // Update a single token field of a resource override.
  const updateResourceToken = (
    path: string,
    field: TokenFieldKey,
    key: 'identifier' | 'location',
    value: string
  ) => {
    const existing = mappingFor(path);
    const cfg = toTokenConfig(existing);
    cfg[field] = { ...cfg[field], [key]: value };
    const updated: ResourceMapping = { resource: path, ...fromTokenConfig(cfg) };
    onChangeResourceMappings(
      resourceMappings.some((m) => m.resource === path)
        ? resourceMappings.map((m) => (m.resource === path ? updated : m))
        : [...resourceMappings, updated]
    );
  };

  const effectiveScope = hidePerResource ? 'default' : scope;

  return (
    <Box>
      {!hidePerResource && (
        <ToggleButtonGroup
          exclusive
          size="small"
          value={effectiveScope}
          onChange={(_, v) => v && setScope(v)}
          sx={{ mb: 2 }}
        >
          <ToggleButton value="default">Global</ToggleButton>
          <ToggleButton value="resource">Per Resource</ToggleButton>
        </ToggleButtonGroup>
      )}

      {effectiveScope === 'default' ? (
        <>
          <Stack direction="row" spacing={1} alignItems="flex-start" sx={{ mb: 2.5 }}>
            <Info size={16} style={{ marginTop: 2, flexShrink: 0, opacity: 0.7 }} />
            <Typography variant="body2" color="text.secondary">
              Applies to all resources unless overridden per resource.
            </Typography>
          </Stack>
          <TokenFieldsEditor tokens={defaultTokens} onChange={onChangeDefaultToken} />
        </>
      ) : (
        <Stack spacing={1.5}>
          <Stack direction="row" spacing={1} alignItems="flex-start">
            <Info size={16} style={{ marginTop: 2, flexShrink: 0, opacity: 0.7 }} />
            <Typography variant="body2" color="text.secondary">
              Turn on a resource to give it its own mapping. The rest use the
              default.
            </Typography>
          </Stack>

          <TextField
            size="small"
            fullWidth
            placeholder="Search resources..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />

          {filtered.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
              {resources.length === 0
                ? 'No resources found. Add an OpenAPI specification on the Connection tab.'
                : 'No resources match your search.'}
            </Typography>
          ) : (
            <Box sx={{ maxHeight: 460, overflowY: 'auto', pr: 0.5 }}>
              {filtered.map((r) => {
                const key = `${r.method}-${r.path}`;
                const overridden = isOverridden(r.path);
                const isOpen = openKey === key;
                return (
                  <Box key={key} sx={{ mb: 0.8 }}>
                    <ResourceRow
                      resource={r}
                      selected={overridden}
                      onClick={() => setOpenKey(isOpen ? null : key)}
                      trailing={
                        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                          <FormControlLabel
                            label="Override"
                            labelPlacement="start"
                            onClick={(e) => e.stopPropagation()}
                            control={
                              <Switch
                                size="small"
                                checked={overridden}
                                onChange={(e) =>
                                  toggleOverride(r.path, e.target.checked)
                                }
                              />
                            }
                          />
                          <IconButton
                            size="small"
                            onClick={(e) => {
                              e.stopPropagation();
                              setOpenKey(isOpen ? null : key);
                            }}
                          >
                            {isOpen ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
                          </IconButton>
                        </Box>
                      }
                    />
                    <Collapse in={isOpen} timeout="auto" unmountOnExit>
                      <Box
                        sx={{
                          mt: 1,
                          px: 1.5,
                          py: 1.5,
                          borderRadius: 1,
                          border: '1px solid',
                          borderColor: 'divider',
                          backgroundColor: 'background.paper',
                        }}
                      >
                        {overridden ? (
                          <TokenFieldsEditor
                            tokens={toTokenConfig(mappingFor(r.path))}
                            onChange={(field, k, value) =>
                              updateResourceToken(r.path, field, k, value)
                            }
                          />
                        ) : (
                          <Typography variant="body2" color="text.secondary">
                            Uses the default mapping. Turn on{' '}
                            <strong>Override</strong> to change it.
                          </Typography>
                        )}
                      </Box>
                    </Collapse>
                  </Box>
                );
              })}
            </Box>
          )}
        </Stack>
      )}
    </Box>
  );
}
