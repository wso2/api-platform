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

import { useRef, useState } from 'react';
import { Box, IconButton, Tooltip } from '@wso2/oxygen-ui';
import { Camera, Trash2 } from '@wso2/oxygen-ui-icons-react';
import { defineMessages, useIntl } from 'react-intl';

import {
  useApiThumbnail,
  useDeleteApiThumbnail,
  useUpsertApiThumbnail,
} from '@/api/resources/apiThumbnail';
import { ConfirmDialog } from '@/components/ConfirmDialog';
import { useNotifications } from '@/components/Notifications';
import { useCan } from '@/permissions/useCan';
import { ApiThumbnailAvatar } from './ApiThumbnailAvatar';

const ALLOWED_MIME_TYPES = ['image/jpeg', 'image/png'] as const;
/** Mirror of the server cap (`DefaultThumbnailMaxBytes`). Keep in sync. */
const MAX_BYTES = 1024 * 1024; // 1 MiB

const messages = defineMessages({
  changeThumbnail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.changeThumbnail',
    defaultMessage: 'Change thumbnail',
  },
  removeThumbnail: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.removeThumbnail',
    defaultMessage: 'Remove thumbnail',
  },
  confirmDeleteTitle: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.confirmDeleteTitle',
    defaultMessage: 'Remove API thumbnail',
  },
  confirmDeleteMessage: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.confirmDeleteMessage',
    defaultMessage:
      'Are you sure you want to delete the thumbnail of this API?',
  },
  confirmDeleteButton: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.confirmDeleteButton',
    defaultMessage: 'Remove',
  },
  errorType: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.errorType',
    defaultMessage: 'Only JPEG and PNG images are supported.',
  },
  errorSize: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.errorSize',
    defaultMessage: 'The image must be 1 MB or smaller.',
  },
  uploadSuccess: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.uploadSuccess',
    defaultMessage: 'Thumbnail updated.',
  },
  deleteSuccess: {
    id: 'apiControlPlane.pages.appShell.appShellPages.apis.ApiThumbnailManager.deleteSuccess',
    defaultMessage: 'Thumbnail removed.',
  },
});

type Props = {
  apiType: string;
  apiId: string;
  displayName: string | undefined;
  size: number;
  fontSize?: number | string;
  iconSize?: number;
  /** When true, hides the hover affordance entirely — e.g. gateway-managed APIs. */
  disabled?: boolean;
};

/**
 * Avatar with a hover-only overlay for managing the API's thumbnail.
 */
export function ApiThumbnailManager({
  apiType,
  apiId,
  displayName,
  size,
  fontSize,
  iconSize,
  disabled = false,
}: Props) {
  const intl = useIntl();
  const { notify } = useNotifications();

  const inputRef = useRef<HTMLInputElement>(null);
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const { url: currentUrl } = useApiThumbnail(apiType, apiId);
  const canUpsert = useCan('UpsertAPIThumbnail');
  const canDelete = useCan('DeleteAPIThumbnail');
  const upsertMutation = useUpsertApiThumbnail();
  const deleteMutation = useDeleteApiThumbnail();

  const showOverlay = !disabled && canUpsert;
  const showRemove = showOverlay && canDelete && Boolean(currentUrl);
  const busy = upsertMutation.isPending || deleteMutation.isPending;

  const pickFile = () => inputRef.current?.click();

  const onFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    // Reset the input value so re-picking the same file still fires change.
    event.target.value = '';
    if (!file) return;

    if (!(ALLOWED_MIME_TYPES as readonly string[]).includes(file.type)) {
      notify(intl.formatMessage(messages.errorType), 'error');
      return;
    }
    if (file.size > MAX_BYTES) {
      notify(intl.formatMessage(messages.errorSize), 'error');
      return;
    }
    try {
      await upsertMutation.mutateAsync({ apiType, apiId, file });
      notify(intl.formatMessage(messages.uploadSuccess), 'success');
    } catch {
      /* Global onMutationError surfaces the detail; nothing extra here. */
    }
  };

  const onConfirmDelete = async () => {
    try {
      await deleteMutation.mutateAsync({ apiType, apiId });
      notify(intl.formatMessage(messages.deleteSuccess), 'success');
    } catch {
      /* Global onMutationError handles the toast. */
    } finally {
      setConfirmingDelete(false);
    }
  };

  return (
    <>
      <Box
        sx={{
          flexShrink: 0,
          position: 'relative',
          // Reveal the overlay on hover of the whole group so moving onto the
          // buttons doesn't flicker it away.
          '&:hover .ApiThumbnailManager__overlay': showOverlay
            ? { opacity: 1, pointerEvents: 'auto' }
            : undefined,
          '&:focus-within .ApiThumbnailManager__overlay': showOverlay
            ? { opacity: 1, pointerEvents: 'auto' }
            : undefined,
        }}
      >
        <ApiThumbnailAvatar
          apiType={apiType}
          apiId={apiId}
          displayName={displayName}
          size={size}
          fontSize={fontSize}
          iconSize={iconSize}
        />
        {showOverlay && (
          <Box
            className="ApiThumbnailManager__overlay"
            sx={{
              alignItems: 'center',
              bgcolor: 'rgba(0, 0, 0, 0.5)',
              borderRadius: 1,
              bottom: 0,
              display: 'flex',
              gap: 0.5,
              inset: 0,
              justifyContent: 'center',
              opacity: 0,
              pointerEvents: 'none',
              position: 'absolute',
              transition: 'opacity 150ms ease',
            }}
          >
            <Tooltip title={intl.formatMessage(messages.changeThumbnail)}>
              {/* Span wrapper lets the Tooltip anchor on a disabled button. */}
              <span>
                <IconButton
                  aria-label={intl.formatMessage(messages.changeThumbnail)}
                  disabled={busy}
                  onClick={pickFile}
                  size="small"
                  sx={{ color: 'common.white' }}
                >
                  <Camera size={Math.max(14, Math.round(size / 4))} />
                </IconButton>
              </span>
            </Tooltip>
            {showRemove && (
              <Tooltip title={intl.formatMessage(messages.removeThumbnail)}>
                <span>
                  <IconButton
                    aria-label={intl.formatMessage(messages.removeThumbnail)}
                    disabled={busy}
                    onClick={() => setConfirmingDelete(true)}
                    size="small"
                    sx={{ color: 'common.white' }}
                  >
                    <Trash2 size={Math.max(14, Math.round(size / 4))} />
                  </IconButton>
                </span>
              </Tooltip>
            )}
          </Box>
        )}
      </Box>

      <Box
        accept={ALLOWED_MIME_TYPES.join(',')}
        component="input"
        onChange={onFileChange}
        ref={inputRef}
        sx={{ display: 'none' }}
        type="file"
      />

      <ConfirmDialog
        confirmLabel={intl.formatMessage(messages.confirmDeleteButton)}
        destructive
        loading={deleteMutation.isPending}
        message={intl.formatMessage(messages.confirmDeleteMessage)}
        onCancel={() => setConfirmingDelete(false)}
        onConfirm={onConfirmDelete}
        open={confirmingDelete}
        title={intl.formatMessage(messages.confirmDeleteTitle)}
      />
    </>
  );
}
