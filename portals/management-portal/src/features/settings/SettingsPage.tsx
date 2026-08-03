import { Card, CardContent, PageContent, PageTitle, Typography } from '@wso2/oxygen-ui';
import { useParams } from 'react-router-dom';

export function SettingsPage() {
  const { projectHandler } = useParams();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Settings</PageTitle.Header>
        <PageTitle.SubHeader>Minimal settings overview for {projectHandler}.</PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <Typography>
            Advanced organization admin settings, governance, marketplace, and
            developer portal configuration are intentionally excluded from the
            MVP replacement app.
          </Typography>
        </CardContent>
      </Card>
    </PageContent>
  );
}
