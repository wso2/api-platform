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
  Card,
  FormControl,
  FormHelperText,
  FormLabel,
  MenuItem,
  OutlinedInput,
  PageTitle,
  Paper,
  Select,
  Stack,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import { useEffect, useId, useState } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { ErrorCode, isErrorCode, type ApiError } from '@/api/core/errors';
import {
  useApiDocument,
  useApiDocumentContent,
  useCreateApiDocument,
  useUpdateApiDocument,
  type ApiDocument,
  type ApiDocumentType,
  type UpdateApiDocumentBody,
} from '@/api/resources/apiDocuments';
import { REST_API_TYPE } from '@/api/resources/apiPublications';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { MarkdownView } from '@/components/MarkdownView';
import { useNotifications } from '@/components/Notifications';
import { ErrorState, LoadingState } from '@/components/StateViews';
import { segmentedSwitchSx } from '@/theme/receipes';
import { isTextContent } from './documentContent';
import {
  DEFAULT_DOCUMENT_TYPE,
  DOCUMENT_TYPES,
  MAX_CUSTOM_TYPE_BYTES,
  documentTypeLabel,
  documentTypeName,
  isCustomDocumentType,
  validateCustomType,
  type CustomTypeError,
} from './documentTypes';

const messages = defineMessages({
  createTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.createTitle',
    defaultMessage: 'Create Document',
  },
  createSubtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.createSubtitle',
    defaultMessage: 'Choose a type, name it and write the content in Markdown.',
  },
  editTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.editTitle',
    defaultMessage: 'Edit Document',
  },
  editSubtitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.editSubtitle',
    defaultMessage: 'Update the type, name or Markdown content of this document.',
  },
  back: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.back',
    defaultMessage: 'Back to documents',
  },
  loading: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.loading',
    defaultMessage: 'Loading document',
  },
  notEditable: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.notEditable',
    defaultMessage: 'This document’s format can’t be edited here.',
  },
  loadError: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.loadError',
    defaultMessage: 'Unable to load this document for editing.',
  },
  typeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.typeLabel',
    defaultMessage: 'Document type',
  },
  customTypeLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.customTypeLabel',
    defaultMessage: 'Custom type',
    description: 'Label for the free-text type name shown when the document type is "Other".',
  },
  customTypePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.customTypePlaceholder',
    defaultMessage: 'e.g. Changelog',
  },
  customTypeTooLong: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.customTypeTooLong',
    defaultMessage: 'Keep under {max} bytes (non-Latin letters count as more than one).',
  },
  customTypeInvalid: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.customTypeInvalid',
    defaultMessage: 'Use only letters, numbers, spaces, hyphens and underscores.',
  },
  customTypeReserved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.customTypeReserved',
    defaultMessage: 'This is a reserved document type.',
  },
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.nameLabel',
    defaultMessage: 'Name',
    description: 'Label for the document name field. Noun.',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.namePlaceholder',
    defaultMessage: 'e.g. Getting started',
  },
  contentLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.contentLabel',
    defaultMessage: 'Content',
  },
  contentHint: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.contentHint',
    defaultMessage: 'Markdown supported: headings, lists, links, code blocks and quotes.',
  },
  contentPlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.contentPlaceholder',
    defaultMessage: '# Write your document in Markdown…',
  },
  layout: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.layout',
    defaultMessage: 'Editor layout',
    description: 'Accessible name of the Source / Split / Preview switch.',
  },
  source: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.source',
    defaultMessage: 'Source',
  },
  split: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.split',
    defaultMessage: 'Split',
    description: 'Editor layout showing the Markdown source and its preview side by side.',
  },
  preview: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.preview',
    defaultMessage: 'Preview',
  },
  markdownPane: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.markdownPane',
    defaultMessage: 'Markdown',
  },
  previewEmpty: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.previewEmpty',
    defaultMessage: 'Nothing to preview yet.',
  },
  leaveTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.leaveTitle',
    defaultMessage: 'Unsaved changes',
    description: 'Title of the dialog asking whether to leave the document form without saving.',
  },
  leaveMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.leaveMessage',
    defaultMessage: 'You have unsaved changes. Are you sure you want to leave?',
  },
  leaveYes: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.leaveYes',
    defaultMessage: 'Yes',
    description: 'Answers "Are you sure you want to leave?": leaves the form and discards unsaved changes.',
  },
  leaveNo: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.leaveNo',
    defaultMessage: 'No',
    description: 'Answers "Are you sure you want to leave?": stays on the form with the changes kept.',
  },
  cancel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.cancel',
    defaultMessage: 'Cancel',
  },
  create: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.create',
    defaultMessage: 'Create',
    description: 'Submits the new document. Verb.',
  },
  save: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.save',
    defaultMessage: 'Save',
    description: 'Saves changes to the document. Verb.',
  },
  saving: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.saving',
    defaultMessage: 'Saving…',
  },
  created: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.created',
    defaultMessage: 'Created "{name}".',
  },
  saved: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.saved',
    defaultMessage: 'Saved "{name}".',
  },
  tooLarge: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.tooLarge',
    defaultMessage: 'The document is too large to save.',
  },
  nameExists: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.nameExists',
    defaultMessage: 'A document with this name already exists for this API.',
  },
});

/**
 * Format problems only. A *missing* required field shows no error — Create /
 * Save simply stays disabled until every required field has a value.
 */
const CUSTOM_TYPE_ERROR_MESSAGE: Record<
  Exclude<CustomTypeError, 'required'>,
  typeof messages.customTypeTooLong
> = {
  tooLong: messages.customTypeTooLong,
  invalid: messages.customTypeInvalid,
  reserved: messages.customTypeReserved,
};

type Layout = 'source' | 'split' | 'preview';

/**
 * One height for every single-line field in the top row. A small Select and a
 * small OutlinedInput otherwise differ by a few pixels, which shows as the
 * boxes not lining up.
 */
const FIELD_SX = { height: 40 } as const;

/** Shared height of the Markdown editor and the preview; both scroll inside it. */
const EDITOR_HEIGHT = 480;

type DocumentEditorProps = {
  apiHandle: string;
  /** The `{apiType}` path segment — `rest-api` unless another artifact kind owns the documents. */
  apiType?: string;
  /** The document to edit; absent for a new document. */
  docId?: string;
  onCancel: () => void;
  onSaved: (docId: string) => void;
};

/**
 * Create or edit form. In edit mode, loads the document's metadata and body
 * (two requests) before showing the form, so it never opens half-filled.
 */
export function DocumentEditor({
  apiHandle,
  apiType = REST_API_TYPE,
  docId,
  onCancel,
  onSaved,
}: DocumentEditorProps) {
  const intl = useIntl();
  const documentQuery = useApiDocument(apiType, apiHandle, docId);
  const contentQuery = useApiDocumentContent(apiType, apiHandle, docId);

  if (docId) {
    if (documentQuery.isPending || contentQuery.isPending) {
      return <LoadingState label={intl.formatMessage(messages.loading)} />;
    }
    if (documentQuery.error || contentQuery.error) {
      return <ErrorState message={intl.formatMessage(messages.loadError)} />;
    }
    // Saving a non-text body back as Markdown would corrupt it.
    if (!isTextContent(contentQuery.data.contentType)) {
      return <ErrorState message={intl.formatMessage(messages.notEditable)} />;
    }
  }

  return (
    <DocumentForm
      apiHandle={apiHandle}
      apiType={apiType}
      existing={docId ? documentQuery.data : undefined}
      existingContent={docId ? contentQuery.data?.text : undefined}
      // Re-seed the form when switching documents rather than carrying edits across.
      key={docId ?? 'new'}
      onCancel={onCancel}
      onSaved={onSaved}
    />
  );
}

type DocumentFormProps = {
  apiHandle: string;
  apiType: string;
  existing?: ApiDocument;
  /** The existing document's body; fetched separately from its metadata. */
  existingContent?: string;
  onCancel: () => void;
  onSaved: (docId: string) => void;
};

type FieldErrors = { displayName?: string; inlineContent?: string; type?: string };

function DocumentForm({
  apiHandle,
  apiType,
  existing,
  existingContent = '',
  onCancel,
  onSaved,
}: DocumentFormProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const createMutation = useCreateApiDocument();
  const updateMutation = useUpdateApiDocument();
  const typeLabelId = useId();
  const nameId = useId();
  const contentId = useId();
  const customTypeId = useId();

  // Only used when creating: an existing document's type is fixed and is
  // shown read-only straight from the response.
  const [type, setType] = useState<ApiDocumentType>(DEFAULT_DOCUMENT_TYPE);
  const [customType, setCustomType] = useState('');
  const [name, setName] = useState(existing?.displayName ?? '');
  const [content, setContent] = useState(existingContent);
  const [layout, setLayout] = useState<Layout>('split');
  const [confirmLeave, setConfirmLeave] = useState(false);
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});

  const isEdit = Boolean(existing);
  const saving = createMutation.isPending || updateMutation.isPending;
  const trimmedName = name.trim();

  const errors: FieldErrors = serverErrors;

  const isOther = type === 'Other';
  const customTypeError = isOther ? validateCustomType(customType) : undefined;
  // A format problem (too long, bad characters) is shown as the user types; an
  // empty value is not an error to show, it just keeps the button disabled.
  const customTypeFormatError = customTypeError === 'required' ? undefined : customTypeError;
  const contentChanged = content !== existingContent;
  const dirty = isEdit ? trimmedName !== existing!.displayName || contentChanged : true;
  const hasUnsavedChanges = isEdit
    ? dirty
    : Boolean(trimmedName || content.trim() || customType.trim()) || type !== DEFAULT_DOCUMENT_TYPE;

  // Cancel and Back ask first when there is something to lose.
  const requestCancel = () => {
    if (hasUnsavedChanges) setConfirmLeave(true);
    else onCancel();
  };

  // Closing or reloading the tab gets the browser's own "leave site?" prompt.
  useEffect(() => {
    if (!hasUnsavedChanges) return undefined;
    const warn = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener('beforeunload', warn);
    return () => window.removeEventListener('beforeunload', warn);
  }, [hasUnsavedChanges]);

  const valid = Boolean(trimmedName) && Boolean(content.trim()) && !customTypeFormatError;

  const handleError = (error: ApiError) => {
    const fieldErrors: FieldErrors = {};
    if (isErrorCode(error, ErrorCode.API_DOCUMENT_NAME_EXISTS)) {
      fieldErrors.displayName = intl.formatMessage(messages.nameExists);
    }
    for (const fieldError of error.fieldErrors) {
      if (fieldError.field === 'displayName' || fieldError.field === 'type') {
        fieldErrors[fieldError.field] = fieldError.message;
      } else if (fieldError.field === 'inlineContent' || fieldError.field === 'file') {
        fieldErrors.inlineContent = fieldError.message;
      }
    }
    setServerErrors(fieldErrors);
    if (isErrorCode(error, 'PAYLOAD_TOO_LARGE')) {
      notify(intl.formatMessage(messages.tooLarge), 'error');
    }
  };

  const save = () => {
    setServerErrors({});
    if (!valid || saving) return;

    if (!existing) {
      createMutation.mutate(
        {
          apiId: apiHandle,
          apiType,
          body: {
            displayName: trimmedName,
            inlineContent: content,
            type,
            otherTypeName: isOther ? customType.trim() || undefined : undefined,
          },
        },
        {
          onError: handleError,
          onSuccess: (created) => {
            notify(intl.formatMessage(messages.created, { name: created.displayName }), 'success');
            onSaved(created.id);
          },
        },
      );
      return;
    }

    const existingIsCustomType = isCustomDocumentType(existing.type);
    const body: UpdateApiDocumentBody = {
      displayName: trimmedName,
      type: existingIsCustomType ? 'Other' : (existing.type as ApiDocumentType),
      otherTypeName: existingIsCustomType ? existing.type : undefined,
      inlineContent: contentChanged ? content : undefined,
    };
    updateMutation.mutate(
      { apiId: apiHandle, apiType, body, docId: existing.id },
      {
        onError: handleError,
        onSuccess: (updated) => {
          notify(intl.formatMessage(messages.saved, { name: updated.displayName }), 'success');
          onSaved(updated.id);
        },
      },
    );
  };

  const showEditor = layout !== 'preview';
  const showPreview = layout !== 'source';

  return (
    <Stack spacing={2}>
      <PageTitle>
        <PageTitle.BackButton onClick={requestCancel} sx={{ alignSelf: 'flex-start', ml: -1 }}>
          <FormattedMessage {...messages.back} />
        </PageTitle.BackButton>
        <PageTitle.Header>
          <FormattedMessage {...(isEdit ? messages.editTitle : messages.createTitle)} />
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <FormattedMessage {...(isEdit ? messages.editSubtitle : messages.createSubtitle)} />
        </PageTitle.SubHeader>
      </PageTitle>

      <Paper sx={{ p: 3 }}>
        <Stack spacing={3}>
          <Stack
            direction={{ md: 'row', xs: 'column' }}
            spacing={2}
            sx={{ alignItems: 'flex-start' }}
          >
            <FormControl
              disabled={isEdit}
              error={Boolean(errors.type)}
              sx={{ flexShrink: 0, width: { md: 240, xs: '100%' } }}
            >
              <FormLabel id={typeLabelId}>
                <FormattedMessage {...messages.typeLabel} />
              </FormLabel>
              {isEdit ? (
                <OutlinedInput
                  inputProps={{ 'aria-labelledby': typeLabelId, readOnly: true }}
                  size="small"
                  sx={FIELD_SX}
                  value={documentTypeName(intl, existing!.type)}
                />
              ) : (
                <Select
                  disabled={saving}
                  labelId={typeLabelId}
                  onChange={(event) => setType(event.target.value as ApiDocumentType)}
                  size="small"
                  sx={FIELD_SX}
                  value={type}
                >
                  {DOCUMENT_TYPES.map((option) => (
                    <MenuItem key={option} value={option}>
                      <FormattedMessage {...documentTypeLabel(option)} />
                    </MenuItem>
                  ))}
                </Select>
              )}
              {errors.type && <FormHelperText>{errors.type}</FormHelperText>}
            </FormControl>
            {/* Asked for only when creating; edit shows the custom name in the type field. */}
            {isOther && !isEdit && (
              <FormControl
                error={Boolean(customTypeFormatError)}
                sx={{ flexShrink: 0, width: { md: 220, xs: '100%' } }}
              >
                <FormLabel htmlFor={customTypeId}>
                  <FormattedMessage {...messages.customTypeLabel} />
                </FormLabel>
                <OutlinedInput
                  disabled={saving}
                  id={customTypeId}
                  onChange={(event) => {
                    setCustomType(event.target.value);
                    setServerErrors((current) => ({ ...current, type: undefined }));
                  }}
                  placeholder={intl.formatMessage(messages.customTypePlaceholder)}
                  size="small"
                  sx={FIELD_SX}
                  value={customType}
                />
                {customTypeFormatError && (
                  <FormHelperText>
                    <FormattedMessage
                      {...CUSTOM_TYPE_ERROR_MESSAGE[customTypeFormatError]}
                      values={{ max: MAX_CUSTOM_TYPE_BYTES }}
                    />
                  </FormHelperText>
                )}
              </FormControl>
            )}
            <FormControl
              error={Boolean(errors.displayName)}
              fullWidth
              required
              sx={{ minWidth: 0 }}
            >
              <FormLabel htmlFor={nameId}>
                <FormattedMessage {...messages.nameLabel} />
              </FormLabel>
              <OutlinedInput
                disabled={saving}
                id={nameId}
                onChange={(event) => {
                  setName(event.target.value);
                  setServerErrors((current) => ({ ...current, displayName: undefined }));
                }}
                placeholder={intl.formatMessage(messages.namePlaceholder)}
                size="small"
                sx={FIELD_SX}
                value={name}
              />
              {errors.displayName && <FormHelperText>{errors.displayName}</FormHelperText>}
            </FormControl>
          </Stack>

          <FormControl error={Boolean(errors.inlineContent)} fullWidth required>
            <Stack
              direction={{ sm: 'row', xs: 'column' }}
              spacing={1.5}
              sx={{ alignItems: { sm: 'center' }, justifyContent: 'space-between', mb: 1 }}
            >
              <Stack spacing={0.25}>
                <FormLabel htmlFor={contentId}>
                  <FormattedMessage {...messages.contentLabel} />
                </FormLabel>
                <Typography color="text.secondary" variant="caption">
                  <FormattedMessage {...messages.contentHint} />
                </Typography>
              </Stack>
              <ToggleButtonGroup
                aria-label={intl.formatMessage(messages.layout)}
                exclusive
                onChange={(_event, value: Layout | null) => {
                  if (value) setLayout(value);
                }}
                size="small"
                sx={segmentedSwitchSx}
                value={layout}
              >
                <ToggleButton value="source">
                  <FormattedMessage {...messages.source} />
                </ToggleButton>
                <ToggleButton value="split">
                  <FormattedMessage {...messages.split} />
                </ToggleButton>
                <ToggleButton value="preview">
                  <FormattedMessage {...messages.preview} />
                </ToggleButton>
              </ToggleButtonGroup>
            </Stack>

            <Stack
              direction={{ md: 'row', xs: 'column' }}
              spacing={2}
              sx={{ alignItems: 'stretch' }}
            >
              {showEditor && (
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <OutlinedInput
                    disabled={saving}
                    fullWidth
                    id={contentId}
                    inputProps={{ spellCheck: false }}
                    multiline
                    rows={1}
                    onChange={(event) => {
                      setContent(event.target.value);
                      setServerErrors((current) => ({ ...current, inlineContent: undefined }));
                    }}
                    placeholder={intl.formatMessage(messages.contentPlaceholder)}
                    sx={{
                      '& textarea': { height: '100% !important', overflowY: 'auto !important' },
                      alignItems: 'stretch',
                      typography: 'body2',
                      fontFamily: 'monospace',
                      height: EDITOR_HEIGHT,
                    }}
                    value={content}
                  />
                </Box>
              )}
              {showPreview && (
                <Card
                  aria-label={intl.formatMessage(messages.preview)}
                  component="section"
                  sx={{ flex: 1, height: EDITOR_HEIGHT, minWidth: 0, overflowY: 'auto', p: 2.5 }}
                  variant="outlined"
                >
                  <MarkdownView
                    emptyFallback={
                      <Typography color="text.secondary" variant="body2">
                        <FormattedMessage {...messages.previewEmpty} />
                      </Typography>
                    }
                    source={content}
                  />
                </Card>
              )}
            </Stack>
            {errors.inlineContent && <FormHelperText>{errors.inlineContent}</FormHelperText>}
          </FormControl>
        </Stack>
      </Paper>

      {/* Pinned above the footer while the form scrolls; buttons only. */}
      <Box sx={{ bottom: 0, position: 'sticky', zIndex: 10 }}>
        <Card>
          <Stack direction="row" spacing={1} sx={{ justifyContent: 'flex-end', p: 2 }}>
            <Button color="secondary" disabled={saving} onClick={requestCancel} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button disabled={saving || !dirty || !valid} onClick={save} variant="contained">
              {saving ? (
                <FormattedMessage {...messages.saving} />
              ) : (
                <FormattedMessage {...(isEdit ? messages.save : messages.create)} />
              )}
            </Button>
          </Stack>
        </Card>
      </Box>

      <ConfirmDialog
        cancelLabel={intl.formatMessage(messages.leaveNo)}
        confirmLabel={intl.formatMessage(messages.leaveYes)}
        message={intl.formatMessage(messages.leaveMessage)}
        onCancel={() => setConfirmLeave(false)}
        onConfirm={() => {
          setConfirmLeave(false);
          onCancel();
        }}
        open={confirmLeave}
        title={intl.formatMessage(messages.leaveTitle)}
      />
    </Stack>
  );
}
