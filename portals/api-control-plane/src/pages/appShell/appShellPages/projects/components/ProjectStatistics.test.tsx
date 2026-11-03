import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { IntlProvider } from 'react-intl';
import { expect, it, vi } from 'vitest';
import { ProjectStatistics } from './ProjectStatistics';

vi.mock('@/api/resources/restApis', () => ({
  useAllRestApis: () => ({
    isPending: false,
    data: { list: [{ kind: 'RestApi' }], pagination: { total: 1 } },
  }),
}));
vi.mock('@/api/resources/graphqlApis', () => ({
  useAllGraphQLApis: () => ({ isPending: false, data: { list: [], pagination: { total: 0 } } }),
}));

it('clears the selected type when Total is activated with the keyboard', async () => {
  const onTypeFilterChange = vi.fn();
  const user = userEvent.setup();
  render(
    <IntlProvider locale="en">
      <ProjectStatistics selectedType="rest" onTypeFilterChange={onTypeFilterChange} />
    </IntlProvider>,
  );
  const total = screen.getByRole('button', { name: /Total/ });
  expect(total.getAttribute('aria-pressed')).toBe('false');
  act(() => total.focus());
  await user.keyboard('{Enter}');
  expect(onTypeFilterChange).toHaveBeenCalledWith(null);
});
