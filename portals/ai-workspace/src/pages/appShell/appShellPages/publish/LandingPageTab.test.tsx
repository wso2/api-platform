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

import { fireEvent } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { renderWithProviders, screen } from '../../../../test/utils';
import { LandingPageTab, type LandingPageTabProps } from './LandingPageTab';

const REPLACES = /Publishing replaces the MCP Hub’s default section/;
const BLANK = 'A blank landing page is not published.';

const renderTab = (props: LandingPageTabProps) => renderWithProviders(<LandingPageTab {...props} />);

describe('LandingPageTab with no page', () => {
  it('offers to create one from the MCP proxy details', async () => {
    const onCreate = vi.fn();
    const { user } = renderTab({ onCreate });
    expect(screen.getByText('No landing page')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Create from MCP proxy details' }));
    expect(onCreate).toHaveBeenCalledTimes(1);
  });

  it('offers nothing to create in the published view', () => {
    renderTab({ readOnly: true });
    expect(screen.getByText('The MCP Hub shows its default page.')).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('shows no editor, so it is never written over, when the stored page could not be read', () => {
    renderTab({ failed: true, onCreate: vi.fn(), text: undefined });
    expect(screen.getByRole('alert')).toHaveTextContent('Unable to load the landing page.');
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Create from MCP proxy details' })).not.toBeInTheDocument();
  });
});

describe('LandingPageTab editor', () => {
  it('edits the Markdown beside a rendered preview of it', () => {
    const onChange = vi.fn();
    renderTab({ onChange, text: '# Travel' });
    const editor = screen.getByRole('textbox', { name: 'Landing page (Markdown)' });
    expect(editor).toHaveValue('# Travel');
    expect(screen.getByRole('region', { name: 'Preview' })).toHaveTextContent('Travel');

    fireEvent.change(editor, { target: { value: '# Travel\n\nMore' } });
    expect(onChange).toHaveBeenCalledWith('# Travel\n\nMore');
  });

  it('is locked while an action runs', () => {
    renderTab({ disabled: true, text: '# Travel' });
    expect(screen.getByRole('textbox', { name: 'Landing page (Markdown)' })).toBeDisabled();
  });
});

describe('LandingPageTab preview pane', () => {
  it('is named Preview while editing and Published page when read-only', () => {
    const { unmount } = renderTab({ text: '# Travel' });
    expect(screen.getByRole('region', { name: 'Preview' })).toBeInTheDocument();
    unmount();

    renderTab({ readOnly: true, text: '# Travel' });
    expect(screen.getByRole('region', { name: 'Published page' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox')).not.toBeInTheDocument();
  });
});

describe('LandingPageTab notices', () => {
  it('says publishing replaces the default section for a page generated from the details', () => {
    renderTab({ generated: true, text: '## Overview\n' });
    expect(screen.getByText(REPLACES)).toBeInTheDocument();
    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });

  it.each(['', '  \n\t'])('says a blank page is not published, never that it replaces anything (%j)', (text) => {
    renderTab({ generated: true, text });
    expect(screen.getByText(BLANK)).toBeInTheDocument();
    expect(screen.queryByText(REPLACES)).not.toBeInTheDocument();
  });

  it('says nothing for a page that was written or already stored', () => {
    renderTab({ text: '# Travel' });
    expect(screen.queryByText(REPLACES)).not.toBeInTheDocument();
    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });

  it('says nothing in the published view', () => {
    renderTab({ readOnly: true, text: '' });
    expect(screen.queryByText(BLANK)).not.toBeInTheDocument();
  });
});
