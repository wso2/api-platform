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
import { Box, Divider, Stack, Typography } from '@wso2/oxygen-ui';
import type { ReactNode } from 'react';

/**
 * The creation wizard's "How do you want to define your API?" step frame,
 * shared by every API type: the approach tabs across the top, flush on the
 * body they open, and that body split into the approach's form (left) and its
 * preview (right). REST fills it with its contract/scratch forms and the
 * OpenAPI resources preview; GraphQL with its schema/scratch forms and the
 * schema explorer — so both read as the same step.
 */
export type DefineApproach<K extends string> = {
  description: ReactNode;
  icon: ReactNode;
  key: K;
  title: ReactNode;
};

type ApproachTabProps = {
  active: boolean;
  description: ReactNode;
  icon: ReactNode;
  onClick: () => void;
  title: ReactNode;
};

const ApproachTab = ({ active, description, icon, onClick, title }: ApproachTabProps) => (
  <Box
    aria-pressed={active}
    component="button"
    onClick={onClick}
    sx={{
      alignItems: 'center',
      bgcolor: active ? 'action.selected' : 'transparent',
      border: 1,
      borderColor: active ? 'primary.main' : 'divider',
      borderRadius: '8px 8px 0 0',
      color: 'text.primary',
      cursor: 'pointer',
      display: 'flex',
      flex: 1,
      gap: 1.5,
      minHeight: 68,
      px: 2,
      py: 1.25,
      textAlign: 'left',
    }}
    type="button"
  >
    <Box
      sx={{
        alignItems: 'center',
        bgcolor: active ? 'primary.main' : 'action.hover',
        borderRadius: 1,
        color: active ? 'primary.contrastText' : 'text.secondary',
        display: 'flex',
        flexShrink: 0,
        height: 40,
        justifyContent: 'center',
        width: 40,
      }}
    >
      {icon}
    </Box>
    <Stack spacing={0.25} sx={{ minWidth: 0 }}>
      <Typography sx={{ fontWeight: 700 }} variant="body1">
        {title}
      </Typography>
      <Typography color="text.secondary" sx={{ opacity: 0.65 }} variant="body2">
        {description}
      </Typography>
    </Stack>
  </Box>
);

export type DefineApproachLayoutProps<K extends string> = {
  approaches: DefineApproach<K>[];
  /** The selected approach's form. */
  form: ReactNode;
  onChange: (approach: K) => void;
  /** What the form resolves to: a resources preview, a schema explorer, an illustration. */
  preview: ReactNode;
  value: K;
};

export function DefineApproachLayout<K extends string>({
  approaches,
  form,
  onChange,
  preview,
  value,
}: DefineApproachLayoutProps<K>) {
  return (
    <Box>
      <Stack
        direction={{ md: 'row', xs: 'column' }}
        sx={{
          '& > button + button': { ml: { md: '-1px', xs: 0 }, mt: { md: 0, xs: '-1px' } },
        }}
      >
        {approaches.map((approach) => (
          <ApproachTab
            active={approach.key === value}
            description={approach.description}
            icon={approach.icon}
            key={approach.key}
            onClick={() => onChange(approach.key)}
            title={approach.title}
          />
        ))}
      </Stack>
      <Stack
        direction={{ lg: 'row', xs: 'column' }}
        sx={{
          border: 1,
          borderColor: 'primary.main',
          borderRadius: '0 0 8px 8px',
          minHeight: 520,
          mt: '-1px',
          overflow: 'hidden',
        }}
      >
        <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>{form}</Box>
        <Divider
          flexItem
          orientation="vertical"
          sx={{
            borderBottomWidth: { lg: 0, xs: 'thin' },
            borderRightWidth: { lg: 'thin', xs: 0 },
          }}
        />
        <Box sx={{ flex: 1, minWidth: 0, p: 3 }}>{preview}</Box>
      </Stack>
    </Box>
  );
}
