import {
  Card,
  CardContent,
  CodeBlock,
  PageContent,
  PageTitle,
} from '@wso2/oxygen-ui';

import { useApiProxy, useApi } from '../../api/hooks/useMvpQueries';
import { ErrorState, LoadingState } from '../../components/StateViews';

export function TestPage() {
  const apiQuery = useApi();
  const apiProxyQuery = useApiProxy(apiQuery.data?.id);

  if (apiQuery.isLoading) return <LoadingState label="Loading test console" />;
  if (!apiQuery.data) return <ErrorState title="API not found" />;

  const context = apiProxyQuery.data?.context || `/${apiQuery.data.name}`;

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>Test {apiQuery.data.displayName}</PageTitle.Header>
        <PageTitle.SubHeader>
          cURL test console for HTTP/API proxies
        </PageTitle.SubHeader>
      </PageTitle>
      <Card variant="outlined">
        <CardContent>
          <CodeBlock
            language="bash"
            code={`curl -X GET "$API_BASE_URL${context}" \\
  -H "Authorization: Bearer <token>"`}
          />
        </CardContent>
      </Card>
    </PageContent>
  );
}
