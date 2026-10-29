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

import type { ReactNode } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';
import { Alert, Box, Button, Card, InputBase, Stack, Typography } from '@wso2/oxygen-ui';
import { Plus } from '@wso2/oxygen-ui-icons-react';
import { MarkdownView } from '../../../../Components/common/MarkdownView';
import { EmptyState, ErrorState } from './PublishStates';
import { hairline } from './publishStyles';

const messages = defineMessages({
  error: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.error',
    defaultMessage: 'Unable to load the landing page.',
  },
  emptyTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.emptyTitle',
    defaultMessage: 'No landing page',
  },
  emptyDescription: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.emptyDescription',
    defaultMessage:
      'The MCP Hub currently shows its default page with your endpoints and capabilities. Add a landing page to replace it with your own content.',
  },
  create: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.create',
    defaultMessage: 'Create from MCP proxy details',
  },
  publishedEmptyDescription: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.publishedEmptyDescription',
    defaultMessage: 'The MCP Hub shows its default page.',
  },
  notice: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.notice',
    defaultMessage:
      'Copied from your MCP proxy details when created. Update it here if they change. Publishing replaces the MCP Hub’s default section.',
  },
  blankNotice: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.blankNotice',
    defaultMessage: 'A blank landing page is not published.',
  },
  editorLabel: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.editorLabel',
    defaultMessage: 'Landing page (Markdown)',
  },
  editorTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.editorTitle',
    defaultMessage: 'Markdown',
  },
  editorPlaceholder: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.editorPlaceholder',
    defaultMessage: '# Write your landing page in Markdown…',
  },
  preview: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.preview',
    defaultMessage: 'Preview',
  },
  previewEmpty: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.previewEmpty',
    defaultMessage: 'Your landing page appears here as you write.',
  },
  publishedTitle: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.publishedTitle',
    defaultMessage: 'Published page',
  },
  publishedEmpty: {
    id: 'aiWorkspace.pages.appShell.appShellPages.publish.landingPage.publishedEmpty',
    defaultMessage: 'Nothing is published yet.',
  },
});

export type LandingPageTabProps = {
  disabled?: boolean;
  /** The stored page could not be read; no editor is offered, so it is not written over. */
  failed?: boolean;
  /** The page is still the generated starter. */
  generated?: boolean;
  onChange?: (text: string) => void;
  /** Starts a page from the proxy's details; absent in the published view. */
  onCreate?: () => void;
  readOnly?: boolean;
  /** `undefined` when there is none yet. */
  text?: string;
};

/** An outlined pane under a title strip; its body scrolls. */
function Pane({ ariaLabel, children, title }: { ariaLabel?: string; children: ReactNode; title: ReactNode }) {
  return (
    <Card
      aria-label={ariaLabel}
      component="section"
      sx={{ display: 'flex', flexDirection: 'column', minHeight: 0, minWidth: 0, overflow: 'hidden' }}
      variant="outlined"
    >
      <Typography
        component="h3"
        sx={(theme) => ({ borderBottom: hairline(theme), borderColor: 'divider', flexShrink: 0, px: 2.5, py: 1.75 })}
        variant="subtitle2"
      >
        {title}
      </Typography>
      {children}
    </Card>
  );
}

/**
 * "Landing Page": the Markdown the MCP Hub shows in place of its default Endpoints and
 * capabilities section, edited beside a preview that only approximates the Hub's styling.
 */
export function LandingPageTab({ disabled, failed, generated, onChange, onCreate, readOnly, text }: LandingPageTabProps) {
  const intl = useIntl();

  if (failed) return <ErrorState message={intl.formatMessage(messages.error)} />;

  if (text === undefined) {
    return (
      <Box component="section" sx={{ p: 3 }}>
        <EmptyState
          action={
            onCreate && (
              <Button onClick={onCreate} startIcon={<Plus size={16} />} variant="contained">
                <FormattedMessage {...messages.create} />
              </Button>
            )
          }
          description={intl.formatMessage(readOnly ? messages.publishedEmptyDescription : messages.emptyDescription)}
          title={intl.formatMessage(messages.emptyTitle)}
        />
      </Box>
    );
  }

  const blank = text.trim() === '';
  const paneTitle = messages[readOnly ? 'publishedTitle' : 'preview'];
  const preview = (
    <Pane ariaLabel={intl.formatMessage(paneTitle)} title={<FormattedMessage {...paneTitle} />}>
      <Box sx={{ flex: 1, minHeight: 0, overflowY: 'auto', p: 3 }}>
        <MarkdownView
          emptyFallback={
            <Alert severity="info">
              <FormattedMessage {...messages[readOnly ? 'publishedEmpty' : 'previewEmpty']} />
            </Alert>
          }
          source={text}
        />
      </Box>
    </Pane>
  );

  if (readOnly) {
    return <Box sx={{ display: 'grid', gridTemplateRows: 'minmax(0, 1fr)', height: '100%', p: 2 }}>{preview}</Box>;
  }

  return (
    <Stack sx={{ height: '100%', overflow: 'hidden' }}>
      {(blank || generated) && (
        <Alert severity="info" sx={{ borderRadius: 0, flexShrink: 0 }}>
          <FormattedMessage {...messages[blank ? 'blankNotice' : 'notice']} />
        </Alert>
      )}
      <Box
        sx={{
          display: 'grid',
          flex: 1,
          gap: 3,
          gridAutoRows: 'minmax(0, 1fr)',
          gridTemplateColumns: 'repeat(auto-fit, minmax(340px, 1fr))',
          minHeight: 0,
          p: 2,
        }}
      >
        <Pane title={<FormattedMessage {...messages.editorTitle} />}>
          <InputBase
            disabled={disabled}
            inputProps={{ 'aria-label': intl.formatMessage(messages.editorLabel), spellCheck: false }}
            multiline
            onChange={(event) => onChange?.(event.target.value)}
            placeholder={intl.formatMessage(messages.editorPlaceholder)}
            rows={1}
            sx={{
              '& textarea': { height: '100% !important', overflowY: 'auto !important', resize: 'none' },
              alignItems: 'stretch',
              flex: 1,
              fontFamily: 'monospace',
              lineHeight: 1.75,
              minHeight: 0,
              p: 2.5,
              typography: 'body2',
            }}
            value={text}
          />
        </Pane>
        {preview}
      </Box>
    </Stack>
  );
}
