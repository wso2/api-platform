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

import { describe, expect, it } from 'vitest';
import { render, screen } from '@testing-library/react';

import {
  Hideable,
  HiddenRegionsProvider,
  SlotEntriesProvider,
  useSlot,
  type SlotEntry,
} from './index';

type TestEntry = SlotEntry & { label: string };

function SlotList({ name }: { name: string }) {
  const entries = useSlot<TestEntry>(name);
  return (
    <ul>
      {entries.map((entry) => (
        <li key={entry.id}>{entry.label}</li>
      ))}
    </ul>
  );
}

describe('useSlot', () => {
  const entries: TestEntry[] = [
    { id: 'b', slot: 'sidebar.project', order: 2, label: 'Second' },
    { id: 'a', slot: 'sidebar.project', order: 1, label: 'First' },
    { id: 'c', slot: 'settings.project.tabs', order: 0, label: 'Other slot' },
  ];

  it('returns only entries for the exact slot name, sorted by order', () => {
    render(
      <SlotEntriesProvider entries={entries}>
        <SlotList name="sidebar.project" />
      </SlotEntriesProvider>
    );

    const items = screen.getAllByRole('listitem').map((li) => li.textContent);
    expect(items).toEqual(['First', 'Second']);
    expect(screen.queryByText('Other slot')).not.toBeInTheDocument();
  });

  it('returns nothing when no provider is mounted', () => {
    render(<SlotList name="sidebar.project" />);
    expect(screen.queryAllByRole('listitem')).toHaveLength(0);
  });
});

describe('Hideable', () => {
  it('renders children when the region is not hidden', () => {
    render(
      <HiddenRegionsProvider hidden={[]}>
        <Hideable name="settings.project.tabs.general">
          <span>General tab</span>
        </Hideable>
      </HiddenRegionsProvider>
    );
    expect(screen.getByText('General tab')).toBeInTheDocument();
  });

  it('suppresses children when the region is named in the hidden set', () => {
    render(
      <HiddenRegionsProvider hidden={['settings.project.tabs.general']}>
        <Hideable name="settings.project.tabs.general">
          <span>General tab</span>
        </Hideable>
      </HiddenRegionsProvider>
    );
    expect(screen.queryByText('General tab')).not.toBeInTheDocument();
  });

  it('renders children when no HiddenRegionsProvider is mounted at all', () => {
    render(
      <Hideable name="settings.project.tabs.general">
        <span>General tab</span>
      </Hideable>
    );
    expect(screen.getByText('General tab')).toBeInTheDocument();
  });
});
