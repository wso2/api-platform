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
  Alert,
  Avatar,
  Box,
  Button,
  Card,
  Checkbox,
  Chip,
  CircularProgress,
  Drawer,
  IconButton,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  CircleAlert,
  ChevronDown,
  ChevronRight,
  Key,
  Search,
  X,
} from "@wso2/oxygen-ui-icons-react";
import type { MappedAPIKey, UserAPIKey } from "../../../../../utils/types";
import {
  type DrawerEntity,
  type SelectionDrawerItemMeta,
  getInitials,
  getKeyStatusColor,
  getVisibleKeys,
} from "./associationsTabUtils";

export type SelectableKeyListProps = {
  keys: UserAPIKey[];
  selectedKeyNames: Set<string>;
  lockedKeyNames?: Set<string>;
  disabledKeyNames?: Set<string>;
  disabledReasonByName?: Map<string, string>;
  keyStatusByName?: Map<string, string | undefined>;
  selectionBlockedMessage?: string | null;
  emptyText: string;
  onToggleKey: (keyName: string) => void;
};

export type AssociationSelectionDrawerProps<T extends DrawerEntity> = {
  open: boolean;
  title: string;
  description: string;
  searchPlaceholder: string;
  searchValue: string;
  onSearchChange: (value: string) => void;
  onClose: () => void;
  isSubmitting: boolean;
  isLoading: boolean;
  loadError: string | null;
  items: T[];
  emptyStateText: string;
  emptySearchText: string;
  linkedIds: Set<string>;
  selectedIds: Set<string>;
  expandedLinkedIds: Set<string>;
  entityKeysMap: Map<string, UserAPIKey[]>;
  mappedKeysMap: Map<string, MappedAPIKey[]>;
  loadingMappedKeyIds: Set<string>;
  loadingEntityKeyIds: Set<string>;
  selectedKeyNamesMap: Map<string, Set<string>>;
  disabledKeyNamesByEntity: Map<string, Set<string>>;
  disabledReasonsByEntity: Map<string, Map<string, string>>;
  selectionBlockedMessage?: string | null;
  onItemClick: (item: T) => Promise<void> | void;
  onToggleKey: (entityId: string, keyName: string) => void;
  getItemMeta: (item: T) => SelectionDrawerItemMeta;
  addButtonLabel: string;
  isAddDisabled: boolean;
  onAdd: () => Promise<void> | void;
};

export function SelectableKeyList({
  keys,
  selectedKeyNames,
  lockedKeyNames = new Set<string>(),
  disabledKeyNames = new Set<string>(),
  disabledReasonByName = new Map(),
  keyStatusByName = new Map(),
  selectionBlockedMessage = null,
  emptyText,
  onToggleKey,
}: SelectableKeyListProps) {
  if (keys.length === 0) {
    return (
      <Typography variant="caption" color="text.secondary">
        {emptyText}
      </Typography>
    );
  }

  return (
    <Stack spacing={0.25}>
      {selectionBlockedMessage ? (
        <Typography variant="caption" color="text.secondary" sx={{ mb: 0.5 }}>
          {selectionBlockedMessage}
        </Typography>
      ) : null}
      {keys.map((key) => {
        const keyName = key.name ?? "";
        const isLocked = lockedKeyNames.has(keyName);
        const isUsedElsewhere = disabledKeyNames.has(keyName);
        const isInteractionBlocked = Boolean(selectionBlockedMessage);
        const isDisabled =
          !isLocked && (isInteractionBlocked || isUsedElsewhere);
        const isSelected = selectedKeyNames.has(keyName);
        const keyStatus = keyStatusByName.get(keyName);
        const disabledReason =
          disabledReasonByName.get(keyName) || selectionBlockedMessage;

        return (
          <Box
            key={keyName}
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1,
              py: 0.75,
              px: 0.5,
              borderRadius: 1,
              cursor: isLocked || isDisabled ? "default" : "pointer",
              opacity: isDisabled ? 0.72 : 1,
              "&:hover":
                isLocked || isDisabled
                  ? undefined
                  : { bgcolor: "action.hover" },
            }}
            onClick={(event) => {
              event.stopPropagation();
              if (isLocked || isDisabled) return;
              onToggleKey(keyName);
            }}
            onKeyDown={(event) => {
              if (isLocked || isDisabled) return;
              if (event.key !== "Enter" && event.key !== " ") return;
              event.preventDefault();
              event.stopPropagation();
              onToggleKey(keyName);
            }}
            role="checkbox"
            aria-checked={isSelected || isUsedElsewhere}
            aria-disabled={isLocked || isDisabled}
            tabIndex={isLocked || isDisabled ? -1 : 0}
          >
            <Checkbox
              checked={isSelected || isUsedElsewhere}
              disabled={isLocked || isDisabled}
              size="small"
              tabIndex={-1}
              disableRipple
              sx={{
                p: 0,
                "&.Mui-disabled": {
                  color: "action.disabled",
                },
                "&.Mui-checked.Mui-disabled": {
                  color: "action.disabled",
                },
              }}
            />
            <Key size={14} />
            <Typography
              variant="caption"
              fontWeight={500}
              noWrap
              sx={{ flex: 1 }}
            >
              {key.name || "—"}
            </Typography>
            {keyStatus ? (
              <Chip
                label={keyStatus}
                size="small"
                variant="outlined"
                color={getKeyStatusColor(keyStatus)}
                sx={{ flexShrink: 0 }}
              />
            ) : null}
            {isUsedElsewhere && !isLocked ? (
              <Stack
                direction="row"
                spacing={0.5}
                alignItems="center"
                sx={{ flexShrink: 0 }}
              >
                <Chip label="Already used" size="small" variant="outlined" />
                <Tooltip
                  title={
                    disabledReason ||
                    "This API key is already used in another application."
                  }
                >
                  <Box
                    component="span"
                    sx={{
                      display: "inline-flex",
                      alignItems: "center",
                      color: "text.secondary",
                    }}
                    onClick={(event) => event.stopPropagation()}
                  >
                    <CircleAlert size={14} />
                  </Box>
                </Tooltip>
              </Stack>
            ) : null}
            {key.artifactType ? (
              <Chip
                label={key.artifactType}
                size="small"
                variant="outlined"
                color="primary"
                sx={{ flexShrink: 0 }}
              />
            ) : null}
          </Box>
        );
      })}
    </Stack>
  );
}

export default function AssociationSelectionDrawer<T extends DrawerEntity>({
  open,
  title,
  description,
  searchPlaceholder,
  searchValue,
  onSearchChange,
  onClose,
  isSubmitting,
  isLoading,
  loadError,
  items,
  emptyStateText,
  emptySearchText,
  linkedIds,
  selectedIds,
  expandedLinkedIds,
  entityKeysMap,
  mappedKeysMap,
  loadingMappedKeyIds,
  loadingEntityKeyIds,
  selectedKeyNamesMap,
  disabledKeyNamesByEntity,
  disabledReasonsByEntity,
  selectionBlockedMessage = null,
  onItemClick,
  onToggleKey,
  getItemMeta,
  addButtonLabel,
  isAddDisabled,
  onAdd,
}: AssociationSelectionDrawerProps<T>) {
  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      sx={{
        "& .MuiDrawer-paper": {
          width: { xs: "100%", sm: 520 },
          maxWidth: "100%",
          display: "flex",
          flexDirection: "column",
        },
      }}
    >
      <Box
        sx={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          p: 2,
          borderBottom: 1,
          borderColor: "divider",
          flexShrink: 0,
        }}
      >
        <Stack spacing={0.25}>
          <Typography variant="h6">{title}</Typography>
          <Typography variant="caption" color="text.secondary">
            {description}
          </Typography>
        </Stack>
        <IconButton onClick={onClose} disabled={isSubmitting} size="small">
          <X size={20} />
        </IconButton>
      </Box>

      <Box sx={{ p: 2, flexShrink: 0, pb: 0 }}>
        <TextField
          fullWidth
          size="small"
          placeholder={searchPlaceholder}
          value={searchValue}
          onChange={(event) => onSearchChange(event.target.value)}
          slotProps={{ input: { startAdornment: <Search size={18} /> } }}
        />
      </Box>

      <Box sx={{ flex: 1, overflow: "auto", p: 2 }}>
        {isLoading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 6 }}>
            <CircularProgress />
          </Box>
        ) : loadError ? (
          <Alert severity="error">{loadError}</Alert>
        ) : items.length === 0 ? (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{ textAlign: "center", py: 4 }}
          >
            {searchValue.trim() ? emptySearchText : emptyStateText}
          </Typography>
        ) : (
          <Stack spacing={1.5}>
            {items.map((item) => {
              const isAlreadyLinked = linkedIds.has(item.id);
              const isSelected = selectedIds.has(item.id);
              const isExpanded = isAlreadyLinked
                ? expandedLinkedIds.has(item.id)
                : isSelected;
              const itemKeys = entityKeysMap.get(item.id) ?? [];
              const mappedKeys = mappedKeysMap.get(item.id) ?? [];
              const isLoadingMappedKeys = loadingMappedKeyIds.has(item.id);
              const isLoadingItemKeys = loadingEntityKeyIds.has(item.id);
              const selectedKeys =
                selectedKeyNamesMap.get(item.id) ?? new Set<string>();
              const mappedKeyStatusMap = new Map(
                mappedKeys.map((key) => [key.keyId, key.status]),
              );
              const lockedKeyNames = new Set(
                Array.from(mappedKeyStatusMap.keys()).filter(Boolean),
              );
              const visibleKeys = getVisibleKeys(
                itemKeys,
                mappedKeys,
                isAlreadyLinked,
              );
              const isKeysLoading = isAlreadyLinked
                ? isLoadingMappedKeys || isLoadingItemKeys
                : isLoadingItemKeys;
              const mergedSelectedKeys = isAlreadyLinked
                ? new Set([
                    ...Array.from(selectedKeys),
                    ...Array.from(lockedKeyNames),
                  ])
                : selectedKeys;
              const itemMeta = getItemMeta(item);

              return (
                <Card
                  key={item.id}
                  sx={{
                    border: 2,
                    borderColor: isExpanded ? "primary.main" : "divider",
                    transition: "border-color 0.15s",
                  }}
                >
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      p: 1.5,
                      gap: 1.5,
                      cursor: "pointer",
                      "&:hover": { bgcolor: "action.hover" },
                    }}
                    onClick={() => void onItemClick(item)}
                    onKeyDown={(event) => {
                      if (event.key !== "Enter" && event.key !== " ") return;
                      event.preventDefault();
                      event.stopPropagation();
                      void onItemClick(item);
                    }}
                    role="button"
                    tabIndex={0}
                    aria-expanded={isExpanded}
                  >
                    <Checkbox
                      checked={isSelected || isAlreadyLinked}
                      disabled={isAlreadyLinked}
                      size="small"
                      tabIndex={-1}
                      disableRipple
                      sx={{ p: 0, flexShrink: 0 }}
                      onClick={(event) => event.stopPropagation()}
                      onChange={() => void onItemClick(item)}
                    />
                    <Avatar
                      sx={{
                        width: 36,
                        height: 36,
                        flexShrink: 0,
                        fontSize: 13,
                        fontWeight: 600,
                        bgcolor: "primary.light",
                        color: "primary.contrastText",
                      }}
                    >
                      {getInitials(item.displayName)}
                    </Avatar>
                    <Stack spacing={0.5} sx={{ minWidth: 0, flex: 1 }}>
                      <Stack
                        direction="row"
                        spacing={1}
                        alignItems="center"
                        flexWrap="wrap"
                      >
                        <Tooltip title={item.displayName.length > 20 ? item.displayName : ""}>
                          <Typography variant="body2" fontWeight={600} noWrap>
                            {item.displayName.length > 30
                              ? `${item.displayName.slice(0, 30)}...`
                              : item.displayName}
                          </Typography>
                        </Tooltip>
                        {itemMeta.chip}
                      </Stack>
                      <Tooltip
                        title={
                          (item.description || "—").length > 20
                            ? item.description || "—"
                            : ""
                        }
                      >
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{
                            display: "-webkit-box",
                            WebkitLineClamp: 2,
                            WebkitBoxOrient: "vertical",
                            overflow: "hidden",
                          }}
                        >
                          {item.description || "—"}
                        </Typography>
                      </Tooltip>
                    </Stack>
                    {isExpanded ? (
                      <ChevronDown size={16} />
                    ) : (
                      <ChevronRight size={16} />
                    )}
                  </Box>

                  {isExpanded ? (
                    <Box
                      sx={{
                        borderTop: 1,
                        borderColor: "divider",
                        px: 1.5,
                        pt: 1,
                        pb: 1.5,
                      }}
                    >
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ display: "block", mb: 1, fontWeight: 600 }}
                      >
                        API Keys
                      </Typography>
                      {isKeysLoading ? (
                        <Box
                          sx={{
                            display: "flex",
                            alignItems: "center",
                            gap: 1,
                            py: 0.5,
                          }}
                        >
                          <CircularProgress size={14} />
                          <Typography variant="caption" color="text.secondary">
                            Loading keys...
                          </Typography>
                        </Box>
                      ) : (
                        <SelectableKeyList
                          keys={visibleKeys}
                          selectedKeyNames={mergedSelectedKeys}
                          lockedKeyNames={lockedKeyNames}
                          disabledKeyNames={
                            disabledKeyNamesByEntity.get(item.id) ??
                            new Set<string>()
                          }
                          disabledReasonByName={
                            disabledReasonsByEntity.get(item.id) ?? new Map()
                          }
                          keyStatusByName={mappedKeyStatusMap}
                          selectionBlockedMessage={selectionBlockedMessage}
                          emptyText={itemMeta.emptyKeysText}
                          onToggleKey={(keyName) =>
                            onToggleKey(item.id, keyName)
                          }
                        />
                      )}
                    </Box>
                  ) : null}
                </Card>
              );
            })}
          </Stack>
        )}
      </Box>

      <Box
        sx={{
          p: 2,
          borderTop: 1,
          borderColor: "divider",
          display: "flex",
          gap: 1,
          flexShrink: 0,
        }}
      >
        <Button
          variant="outlined"
          color="secondary"
          onClick={onClose}
          disabled={isSubmitting}
        >
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => void onAdd()}
          disabled={isAddDisabled}
        >
          {addButtonLabel}
        </Button>
      </Box>
    </Drawer>
  );
}
