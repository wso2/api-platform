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

import type { FC } from 'react';
import { MenuItem, Select } from '@wso2/oxygen-ui';
import type { Environment } from '../types';

export type EnvironmentSelectProps = {
  environments: Environment[];
  value: string;
  onChange: (environmentId: string) => void;
  disabled?: boolean;
};

const EnvironmentSelect: FC<EnvironmentSelectProps> = ({ environments, value, onChange, disabled }) => (
  <Select
    fullWidth
    displayEmpty
    disabled={disabled}
    value={value}
    onChange={(event) => onChange(event.target.value as string)}
    renderValue={(selected) => {
      if (!selected) return 'Select environment';
      return environments.find((environment) => environment.id === selected)?.name ?? String(selected);
    }}
  >
    {environments.map((environment) => (
      <MenuItem key={environment.id} value={environment.id}>
        {environment.name}
      </MenuItem>
    ))}
  </Select>
);

export default EnvironmentSelect;
