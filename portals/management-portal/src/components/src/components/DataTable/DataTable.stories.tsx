import type { Meta, StoryObj } from '@storybook/react';
import { DataTable, DataTableColumn } from './DataTable';
import { Button } from '../Button';
import { Avatar, Box } from '@mui/material';
import { Tooltip } from '../Tooltip';
import { Card, CardContent } from '../Card';
import { SearchBar } from '../SearchBar';
import { useState } from 'react';

const meta: Meta<typeof DataTable> = {
  title: 'Components/Table/DataTable',
  component: DataTable,
  tags: ['autodocs'],
  argTypes: {},
};
export interface IUserGroup {
  id: string;
  uuid: string;
  orgName: string;
  orgUuid: string;
  description: string;
  displayName: string;
  handle: string;
  meta: { created: string; location: string; lastModified: string };
  users: IUser[];
  roles?: any[];
  enterpriseGroups?: string[];
  assignedRoleCount: number;
}

export type UserRoleTag = {
  handle: string;
};

export interface IUserRole {
  id: number;
  uuid: string;
  description: string;
  defaultRole: string;
  displayName: string;
  handle: string;
  tags: UserRoleTag[];
  permissions: Permissions[];
  users?: IUser[];
  assignedGroupCount: number;
}

export interface IUser {
  id: number;
  idpId: string;
  pictureUrl: string;
  email: string;
  displayName: string;
  roles: IUserRole[];
  groups?: IUserGroup[];
}

export default meta;
type Story = StoryObj<typeof DataTable>;

export const Default: Story = {
  args: {},
  render: function RenderDefault(_args) {
    const [searchQuery, setSearchQuery] = useState('');
    const onSearch = (data: any) => {
      setSearchQuery(data);
    };
    const memberData: IUser[] = [
      {
        id: 509,
        idpId: '0525df9c-6585-41be-b88f-24250a7b726d',
        pictureUrl:
          'https://lh3.googleusercontent.com/a-/AOh14GhcotzSbvkVuhkuvwgRE-ctOYmFKMivGX0Sr9bb=s96-c',
        email: 'chathurangada@wso2.com',
        displayName: 'Chathuranga Dassanayake',
        roles: [
          {
            id: 15911,
            uuid: 'f38e45c9-cc13-4b83-9cb1-aa88ecb7ecd1',
            description:
              'Users who develop, deploy, and manage cloud-native applications at scale.',
            displayName: 'Developer',
            handle: 'developer',
            defaultRole: 'developer',
            tags: [{ handle: 'test' }],
            permissions: [],
            assignedGroupCount: 1,
          },
        ],
      },
      {
        id: 1109,
        idpId: '2c5891d5-9e70-4cf4-86d8-fdab954b9a2e',
        pictureUrl:
          'https://lh3.googleusercontent.com/a-/AOh14GgunKlKof5vgZJxHEa1Oql9hiS_U09fmocR4HTV8Q=s96-c',
        email: 'sameeral@wso2.com',
        displayName: 'Sameera Liyanage',
        roles: [
          {
            id: 8283,
            uuid: '8095dfc5-cb6d-476a-9358-d4868df2fa51',
            description: `Users who have full access to
            the Choreo (user management, application development, billing and subscription, etc.)`,
            displayName: 'Admin',
            handle: 'admin',
            defaultRole: 'developer',
            tags: [{ handle: 'test' }],
            permissions: [],
            assignedGroupCount: 1,
          },
        ],
      },
    ];

    const onDeleteMember = (idpId: string, displayName: string) => {
      console.log('Delete member', idpId, displayName);
    };

    const onRowClick = (rowData: IUser) => {
      console.log('Row clicked', rowData);
    };

    const DeleteBtn = ({ onClick }: any) => (
      <Button
        color="error"
        onClick={onClick}
        size="small"
        testId="delete-button"
      >
        Delete
      </Button>
    );

    const renderRoleCountCell = (rowData: IUser, isHover: boolean) => {
      if (isHover && rowData?.roles?.length > 0) {
        return (
          <DeleteBtn
            onClick={(event: any) => {
              event.stopPropagation();
              onDeleteMember(rowData?.idpId, rowData?.displayName);
            }}
          />
        );
      }

      return <span>{`${rowData?.roles.length} role(s)`}</span>;
    };

    const memberListColumns: DataTableColumn<IUser>[] = [
      {
        title: 'Member',
        field: 'displayName',
        width: '25%',
        render: (rowData: IUser) => {
          const { pictureUrl, displayName, email } = rowData;
          return (
            <Box display="flex" alignItems="center" gap={8}>
              {pictureUrl ? (
                <Avatar alt="Member image" src={pictureUrl} />
              ) : (
                <Avatar alt="Member image" src={pictureUrl} />
              )}
              <Box>
                {displayName === 'null' || displayName === null ? (
                  <span>{email}</span>
                ) : (
                  <span>{displayName}</span>
                )}
              </Box>
            </Box>
          );
        },
      },
      {
        title: 'Email',
        field: 'email',
        width: '25%',
      },
      {
        title: 'Roles',
        field: 'roles',
        width: '25%',
        render: (rowData: IUser) => {
          const roles = rowData?.roles?.map((role: any) => role?.displayName);
          return (
            <Tooltip title={roles?.join(',')}>
              <span>{roles?.join(' , ')}</span>
            </Tooltip>
          );
        },
      },
      {
        title: 'Member of',
        field: 'rolesOf',
        align: 'right',
        width: '25%',
        render: (rowData: IUser, isHover: boolean) =>
          renderRoleCountCell(rowData, isHover),
      },
    ];

    return (
      <Card testId="data-table">
        <CardContent>
          <Box display="flex" justifyContent="flex-end">
            <Box width={300}>
              <SearchBar onChange={onSearch} testId="data-table" />
            </Box>
          </Box>
          <DataTable<IUser>
            enableFrontendSearch
            getRowId={(rowData) => rowData.idpId}
            columns={memberListColumns}
            testId="table"
            isLoading={false}
            searchQuery={searchQuery}
            data={memberData}
            totalRows={memberData.length}
            onRowClick={onRowClick}
          />
        </CardContent>
      </Card>
    );
  },
};
