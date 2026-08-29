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
  Sidebar,
  Stack,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import { House, RefreshCw, RotateCcw, TriangleAlert } from '@wso2/oxygen-ui-icons-react';
import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { useNavigate } from 'react-router-dom';

import { runtimeConfig } from '../../config/runtime';
import { isChunkLoadError } from '../../utils/errors/errorClassification';

const messages = defineMessages({
  appBody: {
    id: 'apiControlPlane.components.ErrorFallback.app.body',
    defaultMessage:
      'The console ran into an unexpected problem and could not continue.',
  },
  appTitle: {
    id: 'apiControlPlane.components.ErrorFallback.app.title',
    defaultMessage: 'Something went wrong',
  },
  navigationUnavailable: {
    id: 'apiControlPlane.components.ErrorFallback.chrome.navigationUnavailable',
    defaultMessage:
      'Navigation is unavailable. Reload the page to restore it.',
    description:
      'Tooltip on the marker left in place of the sidebar when it fails to render.',
  },
  goHome: {
    id: 'apiControlPlane.components.ErrorFallback.action.goHome',
    defaultMessage: 'Go to console home',
    description:
      'Leaves the failing page for the console landing page. Escapes a page that fails every time it is opened.',
  },
  pageBody: {
    id: 'apiControlPlane.components.ErrorFallback.page.body',
    defaultMessage:
      'Something went wrong while displaying this page. The rest of the console is still available.',
  },
  pageTitle: {
    id: 'apiControlPlane.components.ErrorFallback.page.title',
    defaultMessage: 'This page could not be displayed',
  },
  reload: {
    id: 'apiControlPlane.components.ErrorFallback.action.reload',
    defaultMessage: 'Reload',
    description: 'Reloads the browser tab. Verb, not a noun.',
  },
  staleBody: {
    id: 'apiControlPlane.components.ErrorFallback.stale.body',
    defaultMessage:
      'This page could not be loaded because the console has been updated since you opened it. Reload to get the latest version.',
    description:
      'Shown when a code-split chunk 404s, which happens to an open tab after a deployment.',
  },
  staleTitle: {
    id: 'apiControlPlane.components.ErrorFallback.stale.title',
    defaultMessage: 'A newer version of the console is available',
  },
  switchersUnavailable: {
    id: 'apiControlPlane.components.ErrorFallback.chrome.switchersUnavailable',
    defaultMessage:
      'Organization, project and API switchers are unavailable. Reload the page to restore them.',
    description:
      'Tooltip on the marker left in place of the header switchers when they fail to render.',
  },
  tryAgain: {
    id: 'apiControlPlane.components.ErrorFallback.action.tryAgain',
    defaultMessage: 'Try again',
    description:
      're-renders the failed area without reloading the browser tab.',
  },
});

/** What every fallback below renders, differing only in copy and actions. */
function ErrorFallbackLayout({
  actions,
  body,
  error,
  title,
}: {
  actions: ReactNode;
  body: ReactNode;
  error: Error;
  title: ReactNode;
}) {
  return (
    <Box
      sx={{
        alignItems: 'center',
        display: 'flex',
        flexDirection: 'column',
        gap: 2,
        justifyContent: 'center',
        minHeight: 360,
        mx: 'auto',
        p: 4,
        textAlign: 'center',
      }}
    >
      <Box sx={{ color: 'warning.main', display: 'flex' }}>
        <TriangleAlert size={48} />
      </Box>
      <Typography variant="h4">{title}</Typography>
      <Typography color="text.secondary">{body}</Typography>

      {/*
        The raw message is a developer artefact ("Cannot read properties of
        undefined"), so it is shown only in a dev build. `componentDidCatch`
        logs it to the console in every build, which is where support should
        read it from.
      */}
      {import.meta.env.DEV && error.message && (
        <Typography
          color="text.disabled"
          sx={{ fontFamily: 'monospace', wordBreak: 'break-word' }}
          variant="caption"
        >
          {error.message}
        </Typography>
      )}

      <Stack direction={{ xs: 'column', sm: 'row' }} spacing={1.5} sx={{ mt: 1 }}>
        {actions}
      </Stack>
    </Box>
  );
}

export type ErrorFallbackProps = {
  error: Error;
  /**
   * Clears the boundary's error state and re-renders its children. Recovers a
   * transient fault without discarding the router, the query cache, or any
   * state held above the boundary — which is what a full reload costs.
   */
  reset: () => void;
};

/**
 * Fallback for the boundary around the routed page, inside `AppLayout`.
 *
 * Deliberately renders only where the page would: the header, sidebar,
 * breadcrumbs and footer stay mounted, so the user navigates away from a broken
 * page instead of being left with a reload button that reproduces the same
 * failure. That is also why "go home" is a real route navigation here rather
 * than a document load.
 */
export function PageErrorFallback({ error, reset }: ErrorFallbackProps) {
  const navigate = useNavigate();

  if (isChunkLoadError(error)) {
    return (
      <ErrorFallbackLayout
        actions={
          <Button
            onClick={() => window.location.reload()}
            startIcon={<RefreshCw size={18} />}
            variant="contained"
          >
            <FormattedMessage {...messages.reload} />
          </Button>
        }
        body={<FormattedMessage {...messages.staleBody} />}
        error={error}
        title={<FormattedMessage {...messages.staleTitle} />}
      />
    );
  }

  return (
    <ErrorFallbackLayout
      actions={
        <>
          <Button
            onClick={reset}
            startIcon={<RotateCcw size={18} />}
            variant="contained"
          >
            <FormattedMessage {...messages.tryAgain} />
          </Button>
          <Button onClick={() => navigate('/')} startIcon={<House size={18} />}>
            <FormattedMessage {...messages.goHome} />
          </Button>
        </>
      }
      body={<FormattedMessage {...messages.pageBody} />}
      error={error}
      title={<FormattedMessage {...messages.pageTitle} />}
    />
  );
}

/**
 * Fallback for the outermost boundary, which sits above `BrowserRouter`.
 *
 * Reached only when something outside the routed page throws, so there is no
 * router to navigate with — "go home" has to be a document load. It points at
 * the app's base path rather than reloading the current URL on purpose: when
 * the failure is deterministic for this route, reloading it reproduces the
 * error forever, and this is the one action that escapes that loop.
 */
export function AppErrorFallback({ error, reset }: ErrorFallbackProps) {
  const stale = isChunkLoadError(error);

  return (
    <ErrorFallbackLayout
      actions={
        <>
          {stale ? (
            <Button
              onClick={() => window.location.reload()}
              startIcon={<RefreshCw size={18} />}
              variant="contained"
            >
              <FormattedMessage {...messages.reload} />
            </Button>
          ) : (
            <Button
              onClick={reset}
              startIcon={<RotateCcw size={18} />}
              variant="contained"
            >
              <FormattedMessage {...messages.tryAgain} />
            </Button>
          )}
          <Button
            onClick={() =>
              window.location.assign(`${runtimeConfig.appBasePath || ''}/`)
            }
            startIcon={<House size={18} />}
          >
            <FormattedMessage {...messages.goHome} />
          </Button>
        </>
      }
      body={
        <FormattedMessage {...(stale ? messages.staleBody : messages.appBody)} />
      }
      error={error}
      title={
        <FormattedMessage
          {...(stale ? messages.staleTitle : messages.appTitle)}
        />
      }
    />
  );
}

/* -------------------------------------------------------------------------- */
/* Persistent chrome                                                          */
/* -------------------------------------------------------------------------- */

/**
 * The marker a failed piece of persistent chrome leaves behind.
 *
 * A header switcher or the sidebar has no room for the full-page treatment
 * above, and no useful action of its own — the surrounding chrome is still
 * working, so the recovery is "carry on, or reload". What it must not do is
 * disappear silently: a console shipped with a missing org switcher and no
 * visible trace is a bug nobody reports. Hence a small, deliberately
 * unobtrusive marker with the explanation in a tooltip.
 *
 * `role="status"` rather than `alert`: this is a degraded region the user may
 * never need, not something demanding immediate attention.
 */
function ChromeErrorMarker({ description }: { description: string }) {
  return (
    <Tooltip title={description}>
      <Box
        aria-label={description}
        role="status"
        sx={{
          alignItems: 'center',
          color: 'text.disabled',
          display: 'inline-flex',
          justifyContent: 'center',
          px: 1,
          py: 0.5,
        }}
      >
        <TriangleAlert size={16} />
      </Box>
    </Tooltip>
  );
}

/**
 * Fallback for the boundary around the header's scope switchers.
 *
 * Scoped to the switchers alone so the rest of the header survives — the brand,
 * the colour-scheme toggle, the notification bell, and above all the user menu
 * with **logout** in it. Losing the ability to sign out because an org lookup
 * returned an unexpected shape is a far worse outcome than losing a switcher.
 */
export function HeaderSwitchersErrorFallback() {
  const intl = useIntl();
  return (
    <ChromeErrorMarker
      description={intl.formatMessage(messages.switchersUnavailable)}
    />
  );
}

/**
 * Fallback for the boundary around the sidebar.
 *
 * Renders a real but empty `Sidebar` rather than nothing, so the rail keeps its
 * width and the shell's grid does not reflow around a missing column. It reads
 * `collapsed`/`width` from `AppShellContext` on its own, so the fallback rail
 * matches whatever the user had before the failure.
 *
 * No synthesised nav items: building a fallback menu would re-run the same
 * scope-dependent logic that just threw. The header's switchers and brand are
 * the way out from here.
 */
export function SidebarErrorFallback() {
  const intl = useIntl();
  return (
    <Sidebar>
      <Sidebar.Nav showDividers={false}>
        <ChromeErrorMarker
          description={intl.formatMessage(messages.navigationUnavailable)}
        />
      </Sidebar.Nav>
    </Sidebar>
  );
}
