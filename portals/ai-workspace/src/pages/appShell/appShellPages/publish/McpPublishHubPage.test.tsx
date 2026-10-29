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

import { fireEvent } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { getApiPortal } from '../../../../apis/apiPortalApis';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { SCOPES } from '../../../../auth/permissions';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { useMcpDeployedGateways } from '../../../../hooks/useMcpDeployedGateways';
import { useMcpProxyDocuments } from '../../../../hooks/useMcpProxyDocuments';
import { useSubscriptionPlans } from '../../../../hooks/useSubscriptionPlans';
import { renderWithProviders, screen, waitFor, within } from '../../../../test/utils';
import McpPublishHubPage from './McpPublishHubPage';

vi.mock('../../../../apis/apiPortalApis', () => ({ getApiPortal: vi.fn() }));
vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: {
    getMCPServer: vi.fn(),
    getMcpProxyApiPortalDraft: vi.fn(),
    getMcpProxyApiPortalPublication: vi.fn(),
    getMcpProxyApiPortalDraftDefinition: vi.fn(),
    getMcpProxyApiPortalPublicationDefinition: vi.fn(),
    saveMcpProxyApiPortalDraft: vi.fn(),
    saveMcpProxyApiPortalDraftDefinition: vi.fn(),
    getMcpProxyApiPortalDraftLandingPage: vi.fn(),
    getMcpProxyApiPortalPublicationLandingPage: vi.fn(),
    saveMcpProxyApiPortalDraftLandingPage: vi.fn(),
    getMcpProxyApiPortalDraftThumbnail: vi.fn(),
    getMcpProxyApiPortalPublicationThumbnail: vi.fn(),
    saveMcpProxyApiPortalDraftThumbnail: vi.fn(),
    publishSavedMcpProxyDraft: vi.fn(),
    unpublishMcpProxyFromApiPortal: vi.fn(),
    deprecateMcpProxyOnApiPortal: vi.fn(),
  },
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({ useAppAuth: vi.fn() }));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({ default: vi.fn() }));
vi.mock('../../../../hooks/useMcpDeployedGateways', () => ({ useMcpDeployedGateways: vi.fn() }));
// jsdom has no object URLs; the blob's size stands in for the URL it would get.
vi.mock('../../../../hooks/useObjectUrl', () => ({
  useObjectUrl: (blob?: Blob) => (blob ? `blob:${blob.size}` : undefined),
}));
vi.mock('../../../../hooks/useMcpProxyDocuments', () => ({ useMcpProxyDocuments: vi.fn() }));
vi.mock('../../../../hooks/useSubscriptionPlans', () => ({ useSubscriptionPlans: vi.fn() }));
vi.mock('../../../../hooks/useFillScrollArea', () => ({
  useFillScrollArea: () => ({ height: undefined, ref: () => undefined }),
}));
vi.mock('../../../../hooks/usePublishProject', () => ({
  usePublishProject: () => ({
    organizationId: 'org-1',
    publishPath: (suffix = '') => `/publish${suffix}`,
  }),
}));
vi.mock('../../../../Components/AILoader', () => ({ default: () => <div data-testid="loader" /> }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const navigate = vi.hoisted(() => vi.fn());
vi.mock('react-router-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}));

const api = vi.mocked(mcpProxiesApis);
const showSnackbar = vi.fn();

const search = { name: 'search', description: 'Find trips' };
const book = { name: 'book', description: 'Book a trip' };
const server = {
  id: 'travel',
  displayName: 'Travel MCP',
  version: '1.0.0',
  description: 'Plans trips.',
  context: '/travel',
  capabilities: { tools: [search, book], resources: [], prompts: [] },
};
const hub = { id: 'main', handle: 'main', name: 'Main Hub', status: 'ACTIVE' };
const gateway = { id: 'gw1', name: 'prod', displayName: 'Prod', endpoints: ['https://gw.example.com'] };
const draft = {
  displayName: 'Travel MCP',
  version: '1.0.0',
  description: 'Plans trips.',
  subscriptionPlanIds: [],
  docIds: [],
  updatedAt: '2026-10-01T00:00:00Z',
};
const live = { ...draft, status: 'PUBLISHED', updatedAt: '2026-10-02T00:00:00Z' };

const notFound = () => Object.assign(new Error('not found'), { status: 404 });
const apiError = (message: string, code?: string, status = 400) => Object.assign(new Error(message), { code, status });

type Answers = {
  draft?: unknown;
  publication?: unknown;
  draftDefinition?: unknown;
  liveDefinition?: unknown;
};

/** What the API holds for this proxy on this hub; whatever is not named is "nothing saved". */
const apiHolds = (answers: Answers = {}) => {
  const answer = (value: unknown) => (value === undefined ? Promise.reject(notFound()) : Promise.resolve(value));
  api.getMCPServer.mockResolvedValue(server as never);
  api.getMcpProxyApiPortalDraft.mockImplementation(() => answer(answers.draft) as never);
  api.getMcpProxyApiPortalPublication.mockImplementation(() => answer(answers.publication) as never);
  api.getMcpProxyApiPortalDraftDefinition.mockImplementation(() => answer(answers.draftDefinition) as never);
  api.getMcpProxyApiPortalPublicationDefinition.mockImplementation(() => answer(answers.liveDefinition) as never);
  api.saveMcpProxyApiPortalDraft.mockResolvedValue({} as never);
  api.saveMcpProxyApiPortalDraftDefinition.mockResolvedValue(undefined);
  api.saveMcpProxyApiPortalDraftLandingPage.mockResolvedValue(undefined);
  api.saveMcpProxyApiPortalDraftThumbnail.mockResolvedValue(undefined);
  api.getMcpProxyApiPortalDraftThumbnail.mockResolvedValue(new Blob(['draft'], { type: 'image/png' }));
  api.getMcpProxyApiPortalPublicationThumbnail.mockResolvedValue(new Blob(['live!'], { type: 'image/png' }));
  api.getMcpProxyApiPortalDraftLandingPage.mockResolvedValue('# Draft page');
  api.getMcpProxyApiPortalPublicationLandingPage.mockResolvedValue('# Live page');
  api.publishSavedMcpProxyDraft.mockResolvedValue({} as never);
  api.unpublishMcpProxyFromApiPortal.mockResolvedValue(undefined);
  api.deprecateMcpProxyOnApiPortal.mockResolvedValue({} as never);
  vi.mocked(getApiPortal).mockResolvedValue(hub as never);
};

const youMay = (...scopes: string[]) =>
  vi.mocked(useAppAuth).mockReturnValue({ hasPermission: (scope: string) => scopes.includes(scope) } as never);

const ALL_SCOPES = [SCOPES.MCP_PROXY_API_PORTAL_MANAGE];

const plansAre = (plans: unknown[]) => vi.mocked(useSubscriptionPlans).mockReturnValue({ plans, failed: false, upToDate: true } as never);

const documentsAre = (documents: unknown[]) =>
  vi.mocked(useMcpProxyDocuments).mockReturnValue({ documents, failed: false, upToDate: true } as never);

const renderPage = async () => {
  const view = renderWithProviders(<McpPublishHubPage />, {
    path: '/publish/:serverId/portals/:apiPortalId',
    route: '/publish/travel/portals/main',
  });
  await screen.findByRole('tab', { name: 'MCP Proxy Details' });
  return view;
};

/** The main side of the split button, whatever it currently says. */
const mainButton = () => screen.getByRole('button', { name: /^(Publish|Republish|Deprecate|Unpublish)$/ });

const pickFromMenu = async (user: Awaited<ReturnType<typeof renderPage>>['user'], item: string) => {
  await user.click(screen.getByRole('button', { name: 'More publish actions' }));
  await user.click(await screen.findByRole('menuitem', { name: item }));
};

const order = (fn: unknown) => (fn as { mock: { invocationCallOrder: number[] } }).mock.invocationCallOrder[0];

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  vi.mocked(getApiPortal).mockReset();
  navigate.mockReset();
  showSnackbar.mockReset();
  vi.mocked(useAIWorkspaceSnackbar).mockReturnValue(showSnackbar as never);
  vi.mocked(useMcpDeployedGateways).mockReturnValue({ gateways: [gateway], isLoading: false } as never);
  plansAre([]);
  documentsAre([]);
  youMay(...ALL_SCOPES);
  apiHolds();
});

describe('McpPublishHubPage', () => {
  describe('opening', () => {
    it('shows a loader until everything it opens on is read', async () => {
      renderWithProviders(<McpPublishHubPage />, {
        path: '/publish/:serverId/portals/:apiPortalId',
        route: '/publish/travel/portals/main',
      });

      expect(screen.getByTestId('loader')).toBeInTheDocument();
      await screen.findByRole('tab', { name: 'MCP Proxy Details' });
    });

    it('titles the page with the proxy and the hub it publishes to', async () => {
      await renderPage();

      expect(screen.getByRole('heading', { name: /Publish\s*Travel MCP/ })).toBeInTheDocument();
      expect(await screen.findByText('Main Hub')).toBeInTheDocument();
    });

    it('starts the form from the proxy itself when nothing was ever saved', async () => {
      await renderPage();

      expect(screen.getByLabelText(/^Name/)).toHaveValue('Travel MCP');
      expect(screen.getByRole('textbox', { name: /^Version/ })).toHaveValue('1.0.0');
      expect(screen.getByLabelText('Production URL')).toHaveValue('https://gw.example.com/travel/mcp');
    });

    it('starts from the saved draft when there is one, and says when it was last edited', async () => {
      apiHolds({ draft: { ...draft, displayName: 'Draft title' } });

      await renderPage();

      expect(screen.getByLabelText(/^Name/)).toHaveValue('Draft title');
      expect(screen.getByText(/^Edited /)).toBeInTheDocument();
    });

    it('goes back to the hub list', async () => {
      await renderPage();

      expect(screen.getByRole('link', { name: 'Back to MCP Hubs' })).toHaveAttribute('href', '/publish/travel');
    });

    it('says so when the page cannot be read, and offers a retry', async () => {
      api.getMCPServer.mockRejectedValue(apiError('Something broke', undefined, 500));

      renderWithProviders(<McpPublishHubPage />, {
        path: '/publish/:serverId/portals/:apiPortalId',
        route: '/publish/travel/portals/main',
      });

      expect(await screen.findByRole('alert')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /retry|try again/i })).toBeInTheDocument();
    });
  });

  describe('Save Draft', () => {
    it('is off until something is edited', async () => {
      await renderPage();

      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    });

    it('saves the details and the definition, but does not publish', async () => {
      const { user } = await renderPage();

      await user.clear(screen.getByLabelText(/^Name/));
      await user.type(screen.getByLabelText(/^Name/), 'Trips');
      expect(screen.getByText('Unsaved changes')).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('Draft saved.', 'success'));
      expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
        'main',
        'travel',
        expect.objectContaining({ displayName: 'Trips', version: '1.0.0' }),
        expect.any(String)
      );
      expect(order(api.saveMcpProxyApiPortalDraft)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftDefinition));
      expect(api.publishSavedMcpProxyDraft).not.toHaveBeenCalled();
      expect(navigate).not.toHaveBeenCalled();
    });

    it('turns off again once what was edited is saved', async () => {
      const { user } = await renderPage();
      await user.type(screen.getByLabelText('Description'), ' More.');

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled());
    });

    it('keeps the draft the server returns rather than reading the page again', async () => {
      api.saveMcpProxyApiPortalDraft.mockResolvedValue({ ...draft, updatedAt: new Date().toISOString() } as never);
      const { user } = await renderPage();
      expect(screen.queryByText(/^Edited /)).not.toBeInTheDocument();
      await user.type(screen.getByLabelText('Description'), ' More.');

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('Draft saved.', 'success'));
      expect(await screen.findByText(/^Edited /)).toBeInTheDocument();
      expect(api.getMCPServer).toHaveBeenCalledTimes(1);
      expect(api.getMcpProxyApiPortalDraft).toHaveBeenCalledTimes(1);
    });

    it('reports a failure from the server and keeps the page as it was', async () => {
      api.saveMcpProxyApiPortalDraft.mockRejectedValue(apiError('Version is not allowed'));
      const { user } = await renderPage();
      await user.type(screen.getByLabelText('Description'), ' More.');

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('Version is not allowed', 'error'));
      expect(screen.getByLabelText('Description')).toHaveValue('Plans trips. More.');
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeEnabled();
      // Read again to show where things stand after the failure.
      await waitFor(() => expect(api.getMCPServer).toHaveBeenCalledTimes(2));
    });

    it('explains a draft that vanished underneath the user in plain words', async () => {
      api.saveMcpProxyApiPortalDraftDefinition.mockRejectedValue(apiError('draft missing', 'DRAFT_NOT_FOUND', 404));
      const { user } = await renderPage();
      await user.type(screen.getByLabelText('Description'), ' More.');

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(showSnackbar).toHaveBeenCalledWith(
          'Unable to save the draft. Your changes are still on this page. Try again.',
          'error'
        )
      );
    });
  });

  describe('Publish', () => {
    it('saves the details, then the definition, then publishes, and returns to the hub list', async () => {
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(navigate).toHaveBeenCalledWith('/publish/travel'));
      expect(showSnackbar).toHaveBeenCalledWith('Published to Main Hub.', 'success');
      expect(order(api.saveMcpProxyApiPortalDraft)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftDefinition));
      expect(order(api.saveMcpProxyApiPortalDraftDefinition)).toBeLessThan(order(api.publishSavedMcpProxyDraft));
      expect(api.publishSavedMcpProxyDraft).toHaveBeenCalledWith('main', 'travel', expect.any(String));
    });

    it('lists everything the proxy offers in the definition it saves when the user has changed nothing', async () => {
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(api.publishSavedMcpProxyDraft).toHaveBeenCalled());
      expect(api.saveMcpProxyApiPortalDraftDefinition).toHaveBeenCalledWith(
        'main',
        'travel',
        { tools: [search, book], resources: [], prompts: [] },
        expect.any(String)
      );
    });

    it('saves only the capabilities that stay ticked', async () => {
      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Capabilities' }));
      await user.click(screen.getByText('Tools'));
      await user.click(screen.getByRole('checkbox', { name: 'book' }));

      await user.click(mainButton());

      await waitFor(() => expect(api.publishSavedMcpProxyDraft).toHaveBeenCalled());
      expect(api.saveMcpProxyApiPortalDraftDefinition).toHaveBeenCalledWith(
        'main',
        'travel',
        { tools: [search], resources: [], prompts: [] },
        expect.any(String)
      );
    });

    it('sends nothing, and shows what is wrong, while the form is invalid', async () => {
      const { user } = await renderPage();
      await user.clear(screen.getByLabelText(/^Name/));

      await user.click(mainButton());

      expect(await screen.findByText('Enter a name.')).toBeInTheDocument();
      expect(api.saveMcpProxyApiPortalDraft).not.toHaveBeenCalled();
      expect(api.publishSavedMcpProxyDraft).not.toHaveBeenCalled();
    });

    it('brings the user back to the details when another tab was open and the form is invalid', async () => {
      const { user } = await renderPage();
      await user.clear(screen.getByLabelText(/^Name/));
      await user.click(screen.getByRole('tab', { name: 'Capabilities' }));

      await user.click(mainButton());

      expect(await screen.findByLabelText(/^Name/)).toBeInTheDocument();
      expect(screen.getByRole('tab', { name: 'MCP Proxy Details' })).toHaveAttribute('aria-selected', 'true');
    });

    it('stays on the page and reports the failure when the server refuses', async () => {
      api.publishSavedMcpProxyDraft.mockRejectedValue(apiError('The hub rejected it', 'PORTAL_ERROR', 502));
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('The hub rejected it', 'error'));
      expect(navigate).not.toHaveBeenCalled();
    });

    it('sends the user to the capabilities when the server says the definition is not publishable', async () => {
      const message = 'The definition needs at least one tool, resource or prompt';
      api.publishSavedMcpProxyDraft.mockRejectedValue(
        Object.assign(apiError(message, 'PUBLICATION_VALIDATION_FAILED'), { errors: [{ field: 'definition', message }] })
      );
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(screen.getByRole('tab', { name: 'Capabilities' })).toHaveAttribute('aria-selected', 'true'));
      expect(showSnackbar).toHaveBeenCalledWith(message, 'error');
    });

    it('stays on the tab it is on when the rejection is about something other than the definition', async () => {
      api.publishSavedMcpProxyDraft.mockRejectedValue(apiError('The selected documents are too large', 'PUBLICATION_VALIDATION_FAILED'));
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('The selected documents are too large', 'error'));
      expect(screen.getByRole('tab', { name: 'MCP Proxy Details' })).toHaveAttribute('aria-selected', 'true');
    });

    it('is off for a user without the manage scope', async () => {
      youMay();

      await renderPage();

      expect(mainButton()).toBeDisabled();
    });

    it('locks the page while it runs', async () => {
      let release: () => void = () => undefined;
      api.publishSavedMcpProxyDraft.mockReturnValue(new Promise((resolve) => (release = () => resolve({} as never))));
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() => expect(screen.getByLabelText(/^Name/)).toBeDisabled());
      expect(mainButton()).toBeDisabled();
      release();
      await waitFor(() => expect(navigate).toHaveBeenCalled());
    });
  });

  describe('subscription plans', () => {
    const gold = { id: 'gold', displayName: 'Gold', status: 'ACTIVE', limits: [] };
    const bronze = { id: 'bronze', displayName: 'Bronze', status: 'INACTIVE', limits: [] };

    it('saves the selection with the draft', async () => {
      plansAre([gold]);
      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));
      await user.click(screen.getByRole('checkbox', { name: /Gold/ }));

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.objectContaining({ subscriptionPlanIds: ['gold'] }),
          expect.any(String)
        )
      );
    });

    it('drops an inactive or missing plan from the selection without a message', async () => {
      apiHolds({ draft: { ...draft, subscriptionPlanIds: ['bronze', 'deleted', 'gold'] } });
      plansAre([gold, bronze]);
      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));

      expect(screen.getByRole('checkbox', { name: /Gold/ })).toHaveAttribute('aria-checked', 'true');
      expect(screen.queryByRole('checkbox', { name: /Bronze/ })).not.toBeInTheDocument();
      expect(showSnackbar).not.toHaveBeenCalled();
      expect(screen.queryByText('Unsaved changes')).not.toBeInTheDocument();
    });

    it('saves and publishes without being stopped by an inactive plan the draft still holds', async () => {
      apiHolds({ draft: { ...draft, subscriptionPlanIds: ['bronze'] } });
      plansAre([gold, bronze]);
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.objectContaining({ subscriptionPlanIds: [] }),
          expect.any(String)
        )
      );
    });

    it('keeps a held plan while the plan list is being read again, since the older list may predate its activation', async () => {
      apiHolds({ draft: { ...draft, subscriptionPlanIds: ['bronze'] } });
      vi.mocked(useSubscriptionPlans).mockReturnValue({ plans: [gold, bronze], failed: false, upToDate: false } as never);
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.objectContaining({ subscriptionPlanIds: ['bronze'] }),
          expect.any(String)
        )
      );
    });

    it('shows the published plans with what became of them since the last publish', async () => {
      apiHolds({
        publication: {
          ...live,
          subscriptionPlanIds: ['gold'],
          subscriptionPlans: [
            { id: 'gold', displayName: 'Gold (at publish)', deleted: false, edited: true },
            { id: 'old', displayName: 'Old plan', deleted: true, edited: false },
          ],
        },
      });
      plansAre([gold]);
      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Subscription Plans' }));

      const goldCard = screen.getByRole('checkbox', { name: /Gold \(at publish\)/ });
      expect(goldCard).toHaveTextContent('Edited since last publish');
      const oldCard = screen.getByRole('checkbox', { name: /Old plan/ });
      expect(oldCard.querySelector('del')).toHaveTextContent('Old plan');
      expect(oldCard).toHaveTextContent('Deleted since last publish');
    });
  });

  describe('the thumbnail', () => {
    const png = (name = 'logo.png', size = 100) => {
      const file = new File(['x'], name, { type: 'image/png' });
      Object.defineProperty(file, 'size', { value: size });
      return file;
    };
    const picker = (container: HTMLElement) => container.querySelector('input[type="file"]') as HTMLInputElement;

    it('shows the monogram until an image is picked, then the image itself', async () => {
      const { container, user } = await renderPage();
      expect(screen.getByText('TR')).toBeInTheDocument();

      await user.upload(picker(container), png());

      expect(screen.getByRole('img', { name: 'Travel MCP' })).toHaveAttribute('src', 'blob:100');
    });

    it('counts a picked image as something to save, and saves it with the draft after the details', async () => {
      const file = png();
      const { container, user } = await renderPage();
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

      await user.upload(picker(container), file);
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraftThumbnail).toHaveBeenCalledWith('main', 'travel', file, expect.any(String))
      );
      expect(order(api.saveMcpProxyApiPortalDraft)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftThumbnail));
    });

    it('is saved before it publishes, and only once the details and definition are in', async () => {
      const { container, user } = await renderPage();
      await user.upload(picker(container), png());

      await user.click(mainButton());

      await waitFor(() => expect(api.publishSavedMcpProxyDraft).toHaveBeenCalled());
      expect(order(api.saveMcpProxyApiPortalDraftDefinition)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftThumbnail));
      expect(order(api.saveMcpProxyApiPortalDraftThumbnail)).toBeLessThan(order(api.publishSavedMcpProxyDraft));
    });

    it('does not publish when the server refuses the image, reports it, and goes back to the monogram', async () => {
      api.saveMcpProxyApiPortalDraftThumbnail.mockRejectedValue(apiError('unsupported image', 'PUBLICATION_VALIDATION_FAILED'));
      const { container, user } = await renderPage();
      await user.upload(picker(container), png());

      await user.click(mainButton());

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('unsupported image', 'error'));
      expect(api.publishSavedMcpProxyDraft).not.toHaveBeenCalled();
      expect(screen.getByText('TR')).toBeInTheDocument();
    });

    it.each([
      ['a GIF', new File(['x'], 'a.gif', { type: 'image/gif' }), 'Only JPEG and PNG images are supported.'],
      ['an image over 2 MB', png('big.png', 2 * 1024 * 1024 + 1), 'The image must be 2 MB or smaller.'],
    ])('refuses %s before uploading it, and says why', async (_name, file, message) => {
      const { container } = await renderPage();

      // Bypasses the picker's own accept filter, which a drag-and-drop or a renamed file can get around.
      fireEvent.change(picker(container), { target: { files: [file] } });

      expect(showSnackbar).toHaveBeenCalledWith(message, 'error');
      expect(screen.getByText('TR')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    });

    it('opens on the draft’s own image, and sends nothing for an image the draft already holds', async () => {
      apiHolds({ draft: { ...draft, hasThumbnail: true } });
      const { user } = await renderPage();
      expect(screen.getByRole('img', { name: 'Travel MCP' })).toHaveAttribute('src', 'blob:5');

      await user.type(screen.getByLabelText(/^Name/), '!');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalled());
      expect(api.saveMcpProxyApiPortalDraftThumbnail).not.toHaveBeenCalled();
    });

    it('pre-fills from the live listing when the draft has none, and sends it to the draft when saved', async () => {
      apiHolds({ draft, publication: { ...live, hasThumbnail: true }, liveDefinition: { tools: [search] } });
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));
      expect(screen.getByRole('img', { name: 'Travel MCP' })).toHaveAttribute('src', 'blob:5');

      await user.type(screen.getByLabelText(/^Name/), '!');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(api.saveMcpProxyApiPortalDraftThumbnail).toHaveBeenCalledTimes(1));
    });

    it('will not save or publish while the live image cannot be read, so the listing never loses it', async () => {
      apiHolds({ publication: { ...live, hasThumbnail: true }, liveDefinition: { tools: [search] } });
      api.getMcpProxyApiPortalPublicationThumbnail.mockRejectedValue(apiError('boom', undefined, 500));
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));

      await user.type(screen.getByLabelText(/^Name/), '!');

      expect(screen.getByText(/thumbnail or landing page could not be loaded/)).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
      expect(mainButton()).toBeDisabled();
    });

    it('still saves the landing page when the thumbnail fails, and does not say the draft was saved', async () => {
      apiHolds({ draft: { ...draft, hasLandingPage: true } });
      api.saveMcpProxyApiPortalDraftThumbnail.mockRejectedValue(apiError('boom', undefined, 500));
      const { container, user } = await renderPage();
      await user.upload(picker(container), png());
      await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
      await user.type(screen.getByRole('textbox', { name: 'Landing page (Markdown)' }), '!');

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('boom', 'error'));
      expect(api.saveMcpProxyApiPortalDraftLandingPage).toHaveBeenCalledWith('main', 'travel', '# Draft page!', expect.any(String));
      expect(showSnackbar).not.toHaveBeenCalledWith('Draft saved.', 'success');
    });

    it('shows the live image read-only in the published view, with no way to change it', async () => {
      apiHolds({
        draft: { ...draft, hasThumbnail: true },
        publication: { ...live, hasThumbnail: true },
        liveDefinition: { tools: [search] },
      });

      await renderPage();

      expect(screen.getByRole('img', { name: 'Travel MCP' })).toHaveAttribute('src', 'blob:5');
      expect(screen.queryByRole('button', { name: 'Change thumbnail' })).not.toBeInTheDocument();
    });
  });

  describe('the landing page', () => {
    const openLandingPage = async () => {
      const view = await renderPage();
      await view.user.click(screen.getByRole('tab', { name: 'Landing Page' }));
      return view;
    };

    it('starts from the proxy’s details, and saves the page with the draft after the details', async () => {
      const { user } = await openLandingPage();
      await user.click(screen.getByRole('button', { name: 'Create from MCP proxy details' }));
      expect((screen.getByRole('textbox', { name: 'Landing page (Markdown)' }) as HTMLTextAreaElement).value).toContain(
        '## Overview\n\nPlans trips.'
      );

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraftLandingPage).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.stringContaining('## Tools'),
          expect.any(String)
        )
      );
      expect(order(api.saveMcpProxyApiPortalDraft)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftLandingPage));
    });

    it('is saved before it publishes, and only once the details and definition are in', async () => {
      const { user } = await openLandingPage();
      await user.click(screen.getByRole('button', { name: 'Create from MCP proxy details' }));

      await user.click(mainButton());

      await waitFor(() => expect(api.publishSavedMcpProxyDraft).toHaveBeenCalled());
      expect(order(api.saveMcpProxyApiPortalDraftDefinition)).toBeLessThan(order(api.saveMcpProxyApiPortalDraftLandingPage));
      expect(order(api.saveMcpProxyApiPortalDraftLandingPage)).toBeLessThan(order(api.publishSavedMcpProxyDraft));
    });

    it('does not publish when the page cannot be saved, and reports it', async () => {
      api.saveMcpProxyApiPortalDraftLandingPage.mockRejectedValue(apiError('too large', 'PAYLOAD_TOO_LARGE', 413));
      const { user } = await openLandingPage();
      await user.click(screen.getByRole('button', { name: 'Create from MCP proxy details' }));

      await user.click(mainButton());

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('too large', 'error'));
      expect(api.publishSavedMcpProxyDraft).not.toHaveBeenCalled();
    });

    it('opens on the draft’s own page, and sends nothing for a page the draft already holds', async () => {
      apiHolds({ draft: { ...draft, hasLandingPage: true } });
      const { user } = await openLandingPage();
      expect(screen.getByRole('textbox', { name: 'Landing page (Markdown)' })).toHaveValue('# Draft page');
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();

      await user.click(screen.getByRole('tab', { name: 'MCP Proxy Details' }));
      await user.type(screen.getByLabelText(/^Name/), '!');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() => expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalled());
      expect(api.saveMcpProxyApiPortalDraftLandingPage).not.toHaveBeenCalled();
    });

    it('counts an edit of the page as something to save', async () => {
      apiHolds({ draft: { ...draft, hasLandingPage: true } });
      const { user } = await openLandingPage();

      await user.type(screen.getByRole('textbox', { name: 'Landing page (Markdown)' }), '!');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraftLandingPage).toHaveBeenCalledWith(
          'main',
          'travel',
          '# Draft page!',
          expect.any(String)
        )
      );
    });

    it('pre-fills from the live listing when the draft has none, and sends it to the draft when saved', async () => {
      apiHolds({ draft, publication: { ...live, hasLandingPage: true }, liveDefinition: { tools: [search] } });
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));
      await user.click(screen.getByRole('tab', { name: 'Landing Page' }));
      expect(screen.getByRole('textbox', { name: 'Landing page (Markdown)' })).toHaveValue('# Live page');

      await user.click(screen.getByRole('tab', { name: 'MCP Proxy Details' }));
      await user.type(screen.getByLabelText(/^Name/), '!');
      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraftLandingPage).toHaveBeenCalledWith('main', 'travel', '# Live page', expect.any(String))
      );
    });

    it('offers no editor, so the stored page is never written over, when it cannot be read', async () => {
      apiHolds({ draft: { ...draft, hasLandingPage: true } });
      api.getMcpProxyApiPortalDraftLandingPage.mockRejectedValue(apiError('boom', undefined, 500));

      await openLandingPage();

      expect(screen.getByText('Unable to load the landing page.')).toBeInTheDocument();
      expect(screen.queryByRole('textbox', { name: 'Landing page (Markdown)' })).not.toBeInTheDocument();
      expect(screen.getByText(/thumbnail or landing page could not be loaded/)).toBeInTheDocument();
      expect(mainButton()).toBeDisabled();
    });

    it('shows the live page read-only in the published view', async () => {
      apiHolds({
        draft: { ...draft, hasLandingPage: true },
        publication: { ...live, hasLandingPage: true },
        liveDefinition: { tools: [search] },
      });
      const { user } = await renderPage();

      await user.click(screen.getByRole('tab', { name: 'Landing Page' }));

      expect(screen.getByRole('region', { name: 'Published page' })).toHaveTextContent('Live page');
      expect(screen.queryByRole('textbox', { name: 'Landing page (Markdown)' })).not.toBeInTheDocument();
    });
  });

  describe('documentation', () => {
    const setup = { id: 'setup', type: 'HowTo', displayName: 'Setup guide' };
    const faq = { id: 'faq', type: 'Other', displayName: 'FAQ list' };

    it('saves the selection with the draft', async () => {
      documentsAre([setup, faq]);
      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Documentation' }));
      await user.click(screen.getByRole('checkbox', { name: 'Setup guide' }));

      await user.click(screen.getByRole('button', { name: 'Save Draft' }));

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.objectContaining({ docIds: ['setup'] }),
          expect.any(String)
        )
      );
    });

    it('only reads the documents once their tab is open or a selection needs checking', async () => {
      const { user } = await renderPage();
      expect(useMcpProxyDocuments).toHaveBeenLastCalledWith('travel', false);

      await user.click(screen.getByRole('tab', { name: 'Documentation' }));

      expect(useMcpProxyDocuments).toHaveBeenLastCalledWith('travel', true);
    });

    it('drops a selected document that no longer exists, without a message', async () => {
      apiHolds({ draft: { ...draft, docIds: ['setup', 'deleted'] } });
      documentsAre([setup]);

      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Documentation' }));

      expect(screen.getByRole('checkbox', { name: 'Setup guide' })).toBeChecked();
      expect(showSnackbar).not.toHaveBeenCalled();
    });

    it('keeps a selected document while the document list is being read again', async () => {
      apiHolds({ draft: { ...draft, docIds: ['setup', 'deleted'] } });
      vi.mocked(useMcpProxyDocuments).mockReturnValue({ documents: [setup], failed: false, upToDate: false } as never);
      const { user } = await renderPage();

      await user.click(mainButton());

      await waitFor(() =>
        expect(api.saveMcpProxyApiPortalDraft).toHaveBeenCalledWith(
          'main',
          'travel',
          expect.objectContaining({ docIds: ['setup', 'deleted'] }),
          expect.any(String)
        )
      );
    });

    it('does not count the removal of a vanished document as an edit to save', async () => {
      apiHolds({ draft: { ...draft, docIds: ['setup', 'deleted'] } });
      documentsAre([setup]);

      const { user } = await renderPage();
      await user.click(screen.getByRole('tab', { name: 'Documentation' }));

      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeDisabled();
    });
  });

  describe('once the proxy is live on the hub', () => {
    beforeEach(() => {
      apiHolds({
        draft: { ...draft, displayName: 'Draft title' },
        publication: { ...live, displayName: 'Live title' },
        draftDefinition: { tools: [search], resources: [], prompts: [] },
        liveDefinition: { tools: [search, book], resources: [], prompts: [] },
      });
    });

    it('shows the published version first, read-only, with no actions to take', async () => {
      await renderPage();

      expect(screen.getByLabelText(/^Name/)).toHaveValue('Live title');
      expect(screen.getByLabelText(/^Name/)).toHaveAttribute('readonly');
      expect(screen.getByText(/^Updated /)).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: 'Save Draft' })).not.toBeInTheDocument();
    });

    it('switches to the draft being edited, which has its own values and the actions', async () => {
      const { user } = await renderPage();

      await user.click(screen.getByRole('button', { name: 'Draft' }));

      expect(screen.getByLabelText(/^Name/)).toHaveValue('Draft title');
      expect(screen.getByRole('button', { name: 'Save Draft' })).toBeInTheDocument();
    });

    it('reads Republish on the main button of the draft', async () => {
      const { user } = await renderPage();

      await user.click(screen.getByRole('button', { name: 'Draft' }));

      expect(screen.getByRole('button', { name: 'Republish' })).toBeInTheDocument();
    });

    it('lists what the live listing holds under Capabilities, without checkboxes', async () => {
      const { user } = await renderPage();

      await user.click(screen.getByRole('tab', { name: 'Capabilities' }));

      expect(await screen.findByText('Total: 2')).toBeInTheDocument();
      expect(screen.queryByRole('checkbox')).not.toBeInTheDocument();
    });

    it('lists only the documents the live listing holds under Documentation', async () => {
      apiHolds({
        draft: { ...draft, docIds: ['setup', 'faq'] },
        publication: { ...live, docIds: ['faq'] },
        liveDefinition: { tools: [search], resources: [], prompts: [] },
      });
      documentsAre([
        { id: 'setup', type: 'HowTo', displayName: 'Setup guide' },
        { id: 'faq', type: 'Other', displayName: 'FAQ list' },
      ]);
      const { user } = await renderPage();

      await user.click(screen.getByRole('tab', { name: 'Documentation' }));

      expect(screen.getByText('FAQ list')).toBeInTheDocument();
      expect(screen.queryByText('Setup guide')).not.toBeInTheDocument();
      expect(screen.getAllByRole('checkbox')).toHaveLength(1);
      expect(screen.getByRole('checkbox', { name: 'FAQ list' })).toBeChecked();
    });

    it('shows the published documents with what became of them since the last publish', async () => {
      apiHolds({
        publication: {
          ...live,
          docIds: ['faq'],
          documents: [
            { id: 'faq', displayName: 'FAQ (at publish)', deleted: false, edited: true },
            { id: 'gone', displayName: 'Old guide', deleted: true, edited: false },
          ],
        },
        liveDefinition: { tools: [search], resources: [], prompts: [] },
      });
      documentsAre([{ id: 'faq', type: 'Other', displayName: 'FAQ list' }]);
      const { user } = await renderPage();

      await user.click(screen.getByRole('tab', { name: 'Documentation' }));

      expect(screen.getByText('Old guide').tagName).toBe('DEL');
      expect(screen.getByText('FAQ (at publish)')).toBeInTheDocument();
      expect(screen.getByText('Edited')).toBeInTheDocument();
    });

    it('opens the draft’s own capabilities, not the live ones', async () => {
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));

      await user.click(screen.getByRole('tab', { name: 'Capabilities' }));

      expect(await screen.findByText('Selected: 1 of 2')).toBeInTheDocument();
    });

    it('unpublishes only after the user types the proxy’s name, then returns to the hub list', async () => {
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));

      await pickFromMenu(user, 'Unpublish');
      await user.click(mainButton());
      const dialog = await screen.findByRole('dialog', { name: 'Unpublish this MCP proxy?' });
      expect(dialog).toHaveTextContent('This removes the MCP proxy "Travel MCP" from Main Hub.');

      const confirm = within(dialog).getByRole('button', { name: 'Confirm' });
      expect(confirm).toBeDisabled();
      await user.type(within(dialog).getByLabelText('Type "Travel MCP" to confirm'), 'Travel MCP');
      await user.click(confirm);

      await waitFor(() => expect(navigate).toHaveBeenCalledWith('/publish/travel'));
      expect(api.unpublishMcpProxyFromApiPortal).toHaveBeenCalledWith('main', 'travel', expect.any(String));
      expect(showSnackbar).toHaveBeenCalledWith('Unpublished from Main Hub.', 'success');
    });

    it('does nothing when the confirmation is cancelled', async () => {
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));
      await pickFromMenu(user, 'Unpublish');
      await user.click(mainButton());

      await user.click(await screen.findByRole('button', { name: 'Cancel' }));

      expect(api.unpublishMcpProxyFromApiPortal).not.toHaveBeenCalled();
    });

    it('deprecates after the same confirmation, and returns to the hub list', async () => {
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));

      await pickFromMenu(user, 'Deprecate');
      await user.click(mainButton());
      const dialog = await screen.findByRole('dialog', { name: 'Deprecate this MCP proxy?' });
      await user.type(within(dialog).getByLabelText('Type "Travel MCP" to confirm'), 'Travel MCP');
      await user.click(within(dialog).getByRole('button', { name: 'Confirm' }));

      await waitFor(() => expect(navigate).toHaveBeenCalledWith('/publish/travel'));
      expect(api.deprecateMcpProxyOnApiPortal).toHaveBeenCalledWith('main', 'travel', expect.any(String));
      expect(showSnackbar).toHaveBeenCalledWith('Deprecated on Main Hub.', 'success');
    });

    it('offers no deprecating of a listing that is already deprecated', async () => {
      apiHolds({
        draft,
        publication: { ...live, status: 'DEPRECATED' },
        draftDefinition: { tools: [] },
        liveDefinition: { tools: [] },
      });
      const { user } = await renderPage();

      expect(screen.getByText(/^Deprecated /)).toBeInTheDocument();
      await user.click(screen.getByRole('button', { name: 'Draft' }));
      await user.click(screen.getByRole('button', { name: 'More publish actions' }));

      expect(await screen.findByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute('aria-disabled', 'true');
      expect(screen.getByRole('menuitem', { name: 'Unpublish' })).not.toHaveAttribute('aria-disabled');
    });

    it('reports a failed unpublish and stays on the page', async () => {
      api.unpublishMcpProxyFromApiPortal.mockRejectedValue(apiError('Hub unavailable', 'PORTAL_ERROR', 502));
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));
      await pickFromMenu(user, 'Unpublish');
      await user.click(mainButton());
      const dialog = await screen.findByRole('dialog', { name: 'Unpublish this MCP proxy?' });
      await user.type(within(dialog).getByLabelText('Type "Travel MCP" to confirm'), 'Travel MCP');

      await user.click(within(dialog).getByRole('button', { name: 'Confirm' }));

      await waitFor(() => expect(showSnackbar).toHaveBeenCalledWith('Hub unavailable', 'error'));
      expect(navigate).not.toHaveBeenCalled();
    });

    it('turns Unpublish and Deprecate off for a user without the manage scope', async () => {
      youMay();
      const { user } = await renderPage();
      await user.click(screen.getByRole('button', { name: 'Draft' }));

      await user.click(screen.getByRole('button', { name: 'More publish actions' }));

      expect(await screen.findByRole('menuitem', { name: 'Deprecate' })).toHaveAttribute('aria-disabled', 'true');
      expect(screen.getByRole('menuitem', { name: 'Unpublish' })).toHaveAttribute('aria-disabled', 'true');
    });
  });
});
