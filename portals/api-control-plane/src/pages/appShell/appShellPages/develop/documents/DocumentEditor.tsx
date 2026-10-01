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
  Chip,
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
import { FileText, Upload } from '@wso2/oxygen-ui-icons-react';
import { useId, useRef, useState, type ChangeEvent } from 'react';
import { defineMessages, FormattedMessage, useIntl } from 'react-intl';

import { isErrorCode, type ApiError } from '@/api/core/errors';
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
import { DEFAULT_DOCUMENT_TYPE, DOCUMENT_TYPES, documentTypeLabel } from './documentTypes';
import { readMarkdownFile, suggestDocumentName, type MarkdownFileError } from './markdownFile';

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
  nameLabel: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.nameLabel',
    defaultMessage: 'Name',
    description: 'Label for the document name field. Noun.',
  },
  namePlaceholder: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.namePlaceholder',
    defaultMessage: 'e.g. Getting started',
  },
  nameRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.nameRequired',
    defaultMessage: 'Enter a name for the document.',
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
    defaultMessage: '# Title\n\nWrite your document in Markdown…',
  },
  contentRequired: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.contentRequired',
    defaultMessage: 'Add some content to the document.',
  },
  layout: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.layout',
    defaultMessage: 'Editor layout',
    description: 'Accessible name of the Write / Split / Preview switch.',
  },
  write: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.write',
    defaultMessage: 'Write',
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
  upload: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.upload',
    defaultMessage: 'Upload',
    description: 'Button that loads a Markdown (.md) file from disk into the editor. Verb.',
  },
  uploadedFile: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.uploadedFile',
    defaultMessage: 'Source file: {fileName}',
    description: 'Accessible label of the chip naming the file the content came from.',
  },
  fileType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.fileType',
    defaultMessage: 'Only Markdown files (.md, .markdown) can be uploaded.',
  },
  fileTooLarge: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.fileTooLarge',
    defaultMessage: 'The file is too large to upload.',
  },
  fileNotText: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.fileNotText',
    defaultMessage: 'The file could not be read as text.',
  },
  overrideTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.overrideTitle',
    defaultMessage: 'Override document content?',
  },
  overrideMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.overrideMessage',
    defaultMessage:
      'Uploading "{fileName}" will replace the current content of "{name}". Any changes made in the editor will be lost.',
  },
  override: {
    id: 'apiControlPlane.pages.appShell.appShellPages.develop.documents.DocumentEditor.override',
    defaultMessage: 'Override',
    description: 'Confirms replacing the document content with an uploaded file. Verb.',
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
});

const FILE_ERROR_MESSAGE: Record<MarkdownFileError, typeof messages.fileType> = {
  type: messages.fileType,
  size: messages.fileTooLarge,
  encoding: messages.fileNotText,
};

type Layout = 'write' | 'split' | 'preview';

/** Shared height of the Markdown editor and the preview; both scroll inside it. */
const EDITOR_HEIGHT = 480;

type DocumentEditorProps = {
  apiHandle: string;
  /** The document to edit; absent for a new document. */
  docId?: string;
  onCancel: () => void;
  onSaved: (docId: string) => void;
};

/**
 * Create or edit form. In edit mode, loads the document's metadata and body
 * (two requests) before showing the form, so it never opens half-filled.
 */
export function DocumentEditor({ apiHandle, docId, onCancel, onSaved }: DocumentEditorProps) {
  const intl = useIntl();
  const documentQuery = useApiDocument(REST_API_TYPE, apiHandle, docId);
  const contentQuery = useApiDocumentContent(REST_API_TYPE, apiHandle, docId);

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
  existing?: ApiDocument;
  /** The existing document's body; fetched separately from its metadata. */
  existingContent?: string;
  onCancel: () => void;
  onSaved: (docId: string) => void;
};

type FieldErrors = { displayName?: string; inlineContent?: string; type?: string };

function DocumentForm({ apiHandle, existing, existingContent = '', onCancel, onSaved }: DocumentFormProps) {
  const intl = useIntl();
  const { notify } = useNotifications();
  const createMutation = useCreateApiDocument();
  const updateMutation = useUpdateApiDocument();
  const fileInput = useRef<HTMLInputElement>(null);
  const typeLabelId = useId();
  const nameId = useId();
  const contentId = useId();

  const [type, setType] = useState<ApiDocumentType>(existing?.type ?? DEFAULT_DOCUMENT_TYPE);
  const [name, setName] = useState(existing?.displayName ?? '');
  const [content, setContent] = useState(existingContent);
  const [fileName, setFileName] = useState(existing?.fileName ?? '');
  const [uploadedNew, setUploadedNew] = useState(false);
  const [layout, setLayout] = useState<Layout>('split');
  const [touched, setTouched] = useState(false);
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});
  const [pendingUpload, setPendingUpload] = useState<{ fileName: string; content: string } | null>(null);

  const isEdit = Boolean(existing);
  const saving = createMutation.isPending || updateMutation.isPending;
  const trimmedName = name.trim();

  const errors: FieldErrors = {
    displayName:
      serverErrors.displayName ??
      (touched && !trimmedName ? intl.formatMessage(messages.nameRequired) : undefined),
    inlineContent:
      serverErrors.inlineContent ??
      (touched && !content.trim() ? intl.formatMessage(messages.contentRequired) : undefined),
    type: serverErrors.type,
  };

  const contentChanged = content !== existingContent;
  const dirty = isEdit
    ? type !== existing!.type || trimmedName !== existing!.displayName || contentChanged
    : true;
  const valid = Boolean(trimmedName) && Boolean(content.trim());

  const applyUpload = (upload: { fileName: string; content: string }) => {
    setContent(upload.content);
    setFileName(upload.fileName);
    setUploadedNew(true);
    setServerErrors((current) => ({ ...current, inlineContent: undefined }));
    if (!trimmedName) setName(suggestDocumentName(upload.content, upload.fileName));
  };

  const onFileChosen = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    // Reset so choosing the same file again still fires a change event.
    event.target.value = '';
    if (!file) return;
    const result = await readMarkdownFile(file);
    if ('error' in result) {
      notify(intl.formatMessage(FILE_ERROR_MESSAGE[result.error]), 'error');
      return;
    }
    // Overwriting a saved document's content is confirmed first; a new
    // document has nothing to lose.
    if (isEdit) setPendingUpload(result);
    else applyUpload(result);
  };

  const handleError = (error: ApiError) => {
    const fieldErrors: FieldErrors = {};
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
    setTouched(true);
    setServerErrors({});
    if (!valid || saving) return;

    if (!existing) {
      createMutation.mutate(
        {
          apiId: apiHandle,
          apiType: REST_API_TYPE,
          body: {
            displayName: trimmedName,
            fileName: fileName || undefined,
            inlineContent: content,
            type,
          },
        },
        {
          onError: handleError,
          onSuccess: (created) => {
            notify(intl.formatMessage(messages.created, { name: created.displayName }), 'success');
            onSaved(created.id);
          },
        }
      );
      return;
    }

    // Only what changed is sent: omitting the content makes this a
    // metadata-only update that leaves the stored bytes untouched.
    const body: UpdateApiDocumentBody = {
      displayName: trimmedName !== existing.displayName ? trimmedName : undefined,
      fileName: uploadedNew && fileName ? fileName : undefined,
      inlineContent: contentChanged ? content : undefined,
      type: type !== existing.type ? type : undefined,
    };
    updateMutation.mutate(
      { apiId: apiHandle, apiType: REST_API_TYPE, body, docId: existing.id },
      {
        onError: handleError,
        onSuccess: (updated) => {
          notify(intl.formatMessage(messages.saved, { name: updated.displayName }), 'success');
          onSaved(updated.id);
        },
      }
    );
  };

  const showEditor = layout !== 'preview';
  const showPreview = layout !== 'write';

  return (
    <Stack spacing={2}>
      <PageTitle>
        <PageTitle.BackButton onClick={onCancel} sx={{ alignSelf: 'flex-start', ml: -1 }}>
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
          <Stack direction={{ md: 'row', xs: 'column' }} spacing={2}>
            <FormControl error={Boolean(errors.type)} sx={{ minWidth: 240 }}>
              <FormLabel id={typeLabelId}>
                <FormattedMessage {...messages.typeLabel} />
              </FormLabel>
              <Select
                disabled={saving}
                labelId={typeLabelId}
                onChange={(event) => setType(event.target.value as ApiDocumentType)}
                size="small"
                value={type}
              >
                {DOCUMENT_TYPES.map((option) => (
                  <MenuItem key={option} value={option}>
                    <FormattedMessage {...documentTypeLabel(option)} />
                  </MenuItem>
                ))}
              </Select>
              {errors.type && <FormHelperText>{errors.type}</FormHelperText>}
            </FormControl>
            <FormControl error={Boolean(errors.displayName)} fullWidth required>
              <FormLabel htmlFor={nameId}>
                <FormattedMessage {...messages.nameLabel} />
              </FormLabel>
              <OutlinedInput
                disabled={saving}
                id={nameId}
                onBlur={() => setTouched(true)}
                onChange={(event) => {
                  setName(event.target.value);
                  setServerErrors((current) => ({ ...current, displayName: undefined }));
                }}
                placeholder={intl.formatMessage(messages.namePlaceholder)}
                size="small"
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
              <Stack direction="row" spacing={1.5} sx={{ alignItems: 'center', flexWrap: 'wrap' }}>
                {fileName && (
                  <Chip
                    aria-label={intl.formatMessage(messages.uploadedFile, { fileName })}
                    icon={<FileText size={14} />}
                    label={fileName}
                    size="small"
                    sx={{ '& .MuiChip-label': { px: 1.25 }, height: 'auto', pl: 0.75, py: 0.5 }}
                  />
                )}
                <Button
                  color="primary"
                  disabled={saving}
                  onClick={() => fileInput.current?.click()}
                  size="small"
                  startIcon={<Upload size={16} />}
                  variant="outlined"
                >
                  <FormattedMessage {...messages.upload} />
                </Button>
                <input
                  accept=".md,.markdown,text/markdown"
                  hidden
                  onChange={(event) => void onFileChosen(event)}
                  ref={fileInput}
                  type="file"
                />
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
                  <ToggleButton value="write">
                    <FormattedMessage {...messages.write} />
                  </ToggleButton>
                  <ToggleButton value="split">
                    <FormattedMessage {...messages.split} />
                  </ToggleButton>
                  <ToggleButton value="preview">
                    <FormattedMessage {...messages.preview} />
                  </ToggleButton>
                </ToggleButtonGroup>
              </Stack>
            </Stack>

            <Stack direction={{ md: 'row', xs: 'column' }} spacing={2} sx={{ alignItems: 'stretch' }}>
              {showEditor && (
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <OutlinedInput
                    disabled={saving}
                    fullWidth
                    id={contentId}
                    inputProps={{ spellCheck: false }}
                    multiline
                    rows={1}
                    onBlur={() => setTouched(true)}
                    onChange={(event) => {
                      setContent(event.target.value);
                      setServerErrors((current) => ({ ...current, inlineContent: undefined }));
                    }}
                    placeholder={intl.formatMessage(messages.contentPlaceholder)}
                    // Fixed height; the textarea fills it and scrolls, rather than growing with the text.
                    sx={{
                      '& textarea': { height: '100% !important', overflowY: 'auto !important' },
                      alignItems: 'stretch',
                      fontFamily: 'monospace',
                      fontSize: '0.8125rem',
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
            <Button color="secondary" disabled={saving} onClick={onCancel} variant="outlined">
              <FormattedMessage {...messages.cancel} />
            </Button>
            <Button
              disabled={saving || !dirty || (touched && !valid)}
              onClick={save}
              variant="contained"
            >
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
        cancelLabel={intl.formatMessage(messages.cancel)}
        confirmColor="warning"
        confirmLabel={intl.formatMessage(messages.override)}
        message={intl.formatMessage(messages.overrideMessage, {
          fileName: pendingUpload?.fileName ?? '',
          name: trimmedName || existing?.displayName || '',
        })}
        onCancel={() => setPendingUpload(null)}
        onConfirm={() => {
          if (pendingUpload) applyUpload(pendingUpload);
          setPendingUpload(null);
        }}
        open={pendingUpload !== null}
        title={intl.formatMessage(messages.overrideTitle)}
      />
    </Stack>
  );
}
