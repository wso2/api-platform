import { Button, Card, CardContent, Stack, TextField, PageContent, PageTitle } from '@wso2/oxygen-ui';

import { useApi } from '../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../components/StateViews';

export function ManagePage() {
  const apiQuery = useApi();

  if (apiQuery.isLoading) return <LoadingState label="Loading manage view" />;
  if (!apiQuery.data) return <ErrorState title="API not found" />;

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Manage {apiQuery.data.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>Core editable metadata only for MVP.</PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <Stack spacing={3}>
            <TextField disabled label="Display name" value={apiQuery.data.displayName} />
            <TextField disabled label="Description" multiline minRows={3} value={apiQuery.data.description || ''} />
            <TextField disabled label="Visibility" value={apiQuery.data.httpBased ? 'HTTP based' : 'Internal'} />
            <Button disabled variant="contained">Save changes</Button>
          </Stack>
        </CardContent>
      </Card>
    </PageContent>
  );
}
