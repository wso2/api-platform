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

import { ListingCard } from '@wso2/oxygen-ui';
import { defineMessages, useIntl } from 'react-intl';

import type { Project } from '@/api/resources/projects';
import { relativeTime } from '@/utils/relativeTime';
import { useCan } from '@/permissions/useCan';

type ProjectCardProps = {
  project: Project;
  onOpen: (project: Project) => void;
  onDelete?: (project: Project) => void;
};

const messages = defineMessages({
  apiCount: {
    id: 'project.card.apiCount',
    defaultMessage: '{count, plural, one {# API} other {# APIs}}',
  },
  apiCountLoading: {
    id: 'project.card.apiCountLoading',
    defaultMessage: '… APIs',
    description: 'Placeholder shown while the API count is still loading.',
  },
  defaultBadge: {
    id: 'project.card.defaultBadge',
    defaultMessage: 'DEFAULT',
    description: 'Badge marking the organization’s default project.',
  },
  delete: {
    id: 'project.card.deleteNamed',
    defaultMessage: 'Delete {name}',
  },
  deployedCount: {
    id: 'project.card.deployedCount',
    defaultMessage: '{count} deployed',
  },
  deployedCountLoading: {
    id: 'project.card.deployedCountLoading',
    defaultMessage: '… deployed',
    description: 'Placeholder shown while the deployed count is still loading.',
  },
  fallbackDescription: {
    id: 'project.card.fallbackDescription',
    defaultMessage: 'No decsription',
    description: 'Shown in place of a description when the project has none.',
  },
  neverUpdated: {
    id: 'project.card.neverUpdated',
    defaultMessage: 'Not updated yet',
  },
  updatedAt: {
    id: 'project.card.updatedAt',
    defaultMessage: 'Updated {relative}',
    description: 'Footer timestamp; {relative} is a phrase such as "3 hours ago".',
  },
});

/** Project card for the grid view, on Oxygen's `ListingCard`. */
export function ProjectCard({ project, onOpen, onDelete }: ProjectCardProps) {
  const intl = useIntl();
  const canDelete = useCan('DeleteProject');

  return (
    <ListingCard
      deleteLabel={intl.formatMessage(messages.delete, { name: project.displayName })}
      description={project.description || intl.formatMessage(messages.fallbackDescription)}
      onClick={() => onOpen(project)}
      onDelete={onDelete && canDelete ? () => onDelete(project) : undefined}
      title={project.displayName}
      updatedDateTime={project.updatedAt || undefined}
      updatedTime={
        project.updatedAt
          ? intl.formatMessage(messages.updatedAt, { relative: relativeTime(project.updatedAt) })
          : intl.formatMessage(messages.neverUpdated)
      }
    />
  );
}
