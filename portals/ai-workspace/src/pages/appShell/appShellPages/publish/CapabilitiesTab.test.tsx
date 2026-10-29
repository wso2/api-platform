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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { describe, expect, it, vi } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import { CapabilitiesTab, type CapabilitiesTabProps } from './CapabilitiesTab';
import { definitionToText } from './mcpPublishForm';

const search = { name: 'search', description: 'Find trips' };
const book = { name: 'book', description: 'Book a trip' };
const guide = { uri: 'file:///guide.md', name: 'Guide' };
const summarize = { name: 'summarize' };

const available = { tools: [search, book], resources: [guide], prompts: [summarize] };

const renderTab = (props: Partial<CapabilitiesTabProps> & { definition?: Record<string, unknown> } = {}) => {
  const { definition = { tools: [search], resources: [], prompts: [] }, ...rest } = props;
  const onChange = vi.fn();
  const view = renderWithProviders(
    <CapabilitiesTab available={available} onChange={onChange} text={definitionToText(definition)} {...rest} />
  );
  return { ...view, onChange };
};

/** The sections start collapsed; opening one is what makes its items reachable. */
const expand = (user: ReturnType<typeof renderTab>['user'], title: 'Tools' | 'Resources' | 'Prompts') =>
  user.click(screen.getByText(title));

/** The definition the last change asked to save. */
const lastSaved = (onChange: ReturnType<typeof vi.fn>) => JSON.parse(onChange.mock.lastCall?.[0] as string);

describe('CapabilitiesTab', () => {
  describe('editing', () => {
    it('explains what to do', () => {
      renderTab();

      expect(
        screen.getByText('Choose the tools, resources and prompts of this MCP proxy to list on the MCP Hub.')
      ).toBeInTheDocument();
    });

    it('has a section for each kind the proxy offers, with its selected count', () => {
      renderTab();

      expect(screen.getByText('Selected: 1 of 2')).toBeInTheDocument();
      expect(screen.getByText('Tools')).toBeInTheDocument();
      expect(screen.getByText('Resources')).toBeInTheDocument();
      expect(screen.getByText('Prompts')).toBeInTheDocument();
      expect(screen.getAllByText('Selected: 0 of 1')).toHaveLength(2);
    });

    it('leaves out a kind that the proxy and the definition both lack', () => {
      renderTab({ available: { tools: [search] }, definition: { tools: [search] } });

      expect(screen.getByText('Tools')).toBeInTheDocument();
      expect(screen.queryByText('Resources')).not.toBeInTheDocument();
      expect(screen.queryByText('Prompts')).not.toBeInTheDocument();
    });

    it('shows each item with its description, ticked when the definition holds it', async () => {
      const { user } = renderTab();
      await expand(user, 'Tools');

      expect(screen.getByRole('checkbox', { name: 'search' })).toBeChecked();
      expect(screen.getByRole('checkbox', { name: 'book' })).not.toBeChecked();
      expect(screen.getByText('Find trips')).toBeInTheDocument();
    });

    it('titles a resource by its URI when it has no name', async () => {
      const { user } = renderTab({ available: { resources: [{ uri: 'file:///raw.txt' }] }, definition: {} });
      await expand(user, 'Resources');

      expect(screen.getByRole('checkbox', { name: 'file:///raw.txt' })).toBeInTheDocument();
    });

    it('selects an item, saving the whole definition with it added', async () => {
      const { onChange, user } = renderTab();
      await expand(user, 'Tools');

      await user.click(screen.getByRole('checkbox', { name: 'book' }));

      expect(lastSaved(onChange)).toEqual({ tools: [search, book], resources: [], prompts: [] });
    });

    it('selects an item by clicking its row, not only the box', async () => {
      const { onChange, user } = renderTab();
      await expand(user, 'Tools');

      await user.click(screen.getByText('Book a trip'));

      expect(lastSaved(onChange).tools).toEqual([search, book]);
    });

    it('deselects an item', async () => {
      const { onChange, user } = renderTab();
      await expand(user, 'Tools');

      await user.click(screen.getByRole('checkbox', { name: 'search' }));

      expect(lastSaved(onChange).tools).toEqual([]);
    });

    it('selects a whole kind at once, leaving the others as they were', async () => {
      const { onChange, user } = renderTab({
        definition: { tools: [search], resources: [guide], prompts: [] },
      });

      await user.click(screen.getByRole('checkbox', { name: 'Select all tools' }));

      expect(lastSaved(onChange)).toEqual({ tools: [search, book], resources: [guide], prompts: [] });
    });

    it('clears a whole kind when all of it is selected', async () => {
      const { onChange, user } = renderTab({ definition: { tools: [search, book], resources: [], prompts: [] } });

      await user.click(screen.getByRole('checkbox', { name: 'Select all tools' }));

      expect(lastSaved(onChange).tools).toEqual([]);
    });

    it('shows a kind as partly selected when only some of it is', () => {
      renderTab();

      expect(screen.getByRole('checkbox', { name: 'Select all tools' })).toHaveAttribute('data-indeterminate', 'true');
    });

    it('turns every checkbox off while an action runs', async () => {
      const { user } = renderTab({ disabled: true });
      await expand(user, 'Tools');

      expect(screen.getByRole('checkbox', { name: 'search' })).toBeDisabled();
      expect(screen.getByRole('checkbox', { name: 'Select all tools' })).toBeDisabled();
    });

    it('flags an item that only the definition holds, and still lets it be cleared', async () => {
      const { onChange, user } = renderTab({ definition: { tools: [search, { name: 'retired' }] } });
      await expand(user, 'Tools');

      expect(screen.getByText('Not on this MCP proxy')).toBeInTheDocument();

      await user.click(screen.getByRole('checkbox', { name: 'retired' }));

      expect(lastSaved(onChange).tools).toEqual([search]);
    });

    it('says there is nothing to list when neither the proxy nor the definition has anything', () => {
      renderTab({ available: {}, definition: {} });

      expect(
        screen.getByText("This MCP proxy has no tools, resources or prompts to list. Add them from the MCP proxy's Overview page.")
      ).toBeInTheDocument();
    });

    it('shows the whole of a name cut short on the first hover', async () => {
      const { user } = renderTab();
      await expand(user, 'Tools');
      const name = screen.getByText('search');
      // jsdom does no layout, so the cut is stated: wider than the room it has.
      Object.defineProperty(name, 'scrollWidth', { configurable: true, value: 200 });
      Object.defineProperty(name, 'clientWidth', { configurable: true, value: 100 });

      await user.hover(name);

      expect(await screen.findByRole('tooltip')).toHaveTextContent('search');
    });

    it('shows no tooltip for a name that fits', async () => {
      const { user } = renderTab();
      await expand(user, 'Tools');

      await user.hover(screen.getByText('search'));

      await expect(screen.findByRole('tooltip', {}, { timeout: 300 })).rejects.toThrow();
    });
  });

  describe('read-only, for the published version', () => {
    const renderReadOnly = (definition: Record<string, unknown>) => {
      const onChange = vi.fn();
      renderWithProviders(<CapabilitiesTab onChange={onChange} readOnly text={definitionToText(definition)} />);
      return onChange;
    };

    it('lists what the definition holds, with a total for each kind', () => {
      renderReadOnly({ tools: [search, book], resources: [guide], prompts: [] });

      expect(screen.getByText('Total: 2')).toBeInTheDocument();
      expect(screen.getByText('Total: 1')).toBeInTheDocument();
      expect(screen.queryByText('Prompts')).not.toBeInTheDocument();
    });

    it('has no checkboxes and no intro', () => {
      renderReadOnly({ tools: [search] });

      expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
      expect(screen.queryByText(/Choose the tools/)).not.toBeInTheDocument();
    });

    it('does not flag an item as missing from the proxy, which it never compares against', () => {
      renderReadOnly({ tools: [search] });

      expect(screen.queryByText('Not on this MCP proxy')).not.toBeInTheDocument();
    });

    it('says so when the version lists nothing', () => {
      renderReadOnly({});

      expect(screen.getByText('This version lists no tools, resources or prompts.')).toBeInTheDocument();
    });
  });
});
