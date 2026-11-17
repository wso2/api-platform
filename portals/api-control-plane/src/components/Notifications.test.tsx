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

import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { Button, Drawer } from '@wso2/oxygen-ui';

import { fireEvent, renderWithProviders, screen } from '@/test/utils';
import { useNotifications } from './Notifications';

function DrawerAction() {
  const { notify } = useNotifications();
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button onClick={() => setOpen(true)}>Open</Button>
      <Drawer open={open} onClose={() => setOpen(false)}>
        <Button onClick={() => notify('Deleted "v1".', 'success')}>Delete</Button>
      </Drawer>
    </>
  );
}

describe('NotificationProvider', () => {
  it('announces a toast raised from inside an open drawer', () => {
    renderWithProviders(<DrawerAction />);

    fireEvent.click(screen.getByRole('button', { name: 'Open' }));
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }));

    // getByRole skips aria-hidden content, as screen readers do.
    expect(screen.getByRole('alert')).toHaveTextContent('Deleted "v1".');
  });
});
