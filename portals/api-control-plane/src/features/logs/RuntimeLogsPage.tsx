import { Card, CardContent, CodeBlock, PageContent, PageTitle } from '@wso2/oxygen-ui';
import { useParams } from 'react-router-dom';

export function RuntimeLogsPage() {
  const { projectHandler } = useParams();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Runtime logs</PageTitle.Header>
        <PageTitle.SubHeader>Project-level logs entry for {projectHandler}.</PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <CodeBlock
            language="bash"
            code={`[info] Runtime log streaming integration point
[info] Advanced filters and live tail are deferred from the MVP`}
          />
        </CardContent>
      </Card>
    </PageContent>
  );
}
