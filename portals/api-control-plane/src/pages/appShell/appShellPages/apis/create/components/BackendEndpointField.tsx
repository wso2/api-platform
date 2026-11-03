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
import {
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  InputAdornment,
  OutlinedInput,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import { Link as LinkIcon, Zap } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';

/**
 * The "Backend endpoint" block of the wizard's Start-from-scratch approach,
 * shared by every API type: heading, explanation, the endpoint field and a
 * "Try with Sample URL" shortcut. An API type adds what is specific to it —
 * GraphQL its introspection status — as `children`, below the shortcut.
 */
export type BackendEndpointFieldProps = {
  children?: ReactNode;
  description: ReactNode;
  /** Shown under the field, which turns to its error state while set. */
  error?: ReactNode;
  heading: ReactNode;
  inputId: string;
  label: ReactNode;
  onBlur?: () => void;
  onChange: (value: string) => void;
  onSample: () => void;
  placeholder?: string;
  required?: boolean;
  sampleLabel: ReactNode;
  value: string;
};

export const BackendEndpointField = ({
  children,
  description,
  error,
  heading,
  inputId,
  label,
  onBlur,
  onChange,
  onSample,
  placeholder,
  required,
  sampleLabel,
  value,
}: BackendEndpointFieldProps) => (
  <Stack spacing={2.5}>
    <Box>
      <Typography sx={{ fontWeight: 700 }} variant="h3">
        {heading}
      </Typography>
      <Typography color="text.secondary" sx={{ mt: 0.5 }} variant="body2">
        {description}
      </Typography>
    </Box>
    <Stack spacing={1}>
      <FormControl error={Boolean(error)} fullWidth required={required}>
        <FormLabel htmlFor={inputId}>{label}</FormLabel>
        <OutlinedInput
          id={inputId}
          onBlur={onBlur}
          onChange={(event) => onChange(event.target.value)}
          placeholder={placeholder}
          startAdornment={
            <InputAdornment position="start">
              <LinkIcon size={18} />
            </InputAdornment>
          }
          sx={{ mt: 0.75 }}
          value={value}
        />
        {error ? <FormHelperText>{error}</FormHelperText> : null}
        <Button
          onClick={onSample}
          size="small"
          startIcon={<Zap size={16} />}
          sx={{ alignSelf: 'flex-start', px: 0, textTransform: 'none' }}
          type="button"
          variant="text"
        >
          {sampleLabel}
        </Button>
      </FormControl>
      {children}
    </Stack>
  </Stack>
);
