# AsyncAPI Viewer

A React component library for viewing and testing AsyncAPI specifications with support for both JSON and YAML formats.

## Features

- WebSocket API testing
- YAML and JSON support

## Usage

The component is distributed via a CDN and can be integrated into your application by including the following <script> tag in your HTML file:

```html
<script src="https://cdn.jsdelivr.net/gh/wso2/api-developer-portal-core@refs/heads/main/libs/async-api-viewer/dist/websocket-viewer.umd.js"></script>
```

### Basic Usage

```html
<!DOCTYPE html>
<html>
<head>
    <script src="https://unpkg.com/react@17.0.2/umd/react.development.js"></script>
    <script src="https://unpkg.com/react-dom@17.0.2/umd/react-dom.development.js"></script>
    <script src="https://cdn.jsdelivr.net/gh/wso2/api-developer-portal-core@refs/heads/main/libs/async-api-viewer/dist/websocket-viewer.umd.js"></script>
</head>
<body>
<div id="root"></div>
<script>
    const asyncapi = {
        asyncapi: '2.0.0',
        info: {
            title: 'My API',
            version: '1.0.0',
            description: 'A sample AsyncAPI specification'
        },
        channels: {
            'test': {
                subscribe: {
                    message: 'A sample message',
                    operationId: 'test'
                },
                publish: {
                    message: 'A sample message',
                    operationId: 'test'
                }
            }
        }
    };
    let serverUrl = 'wss://echo.websocket.org';
    const rootElement = document.getElementById('root');
    ReactDOM.render(React.createElement(WebSocketViewer.default, {
        apiEndpoint: serverUrl,
        token: 'token',
        asyncapi: asyncapi,
    }), rootElement);
</script>
</body>
</html>
```

## Props

### WebSocketViewer

| Prop | Type | Default | Description |
|------|------|---------|-------------|
| `schema` | `string \| AsyncAPISpec` | - | AsyncAPI specification as JSON string, YAML string, or parsed object |
| `targetUrl` | `string` | - | Target URL for the API |
| `token` | `string` | - | Token for the API |


## Development

```bash
# Install dependencies
npm install

# Build the package
npm run build
```

You can test the component using the test.html file.

Commit the dist folder to the repo.

## Dependencies

This package depends on:
- `react-monaco-editor`: For the Monaco editor
- `lodash`: For utility functions
- `@material-ui/core`: For UI components
- `@material-ui/icons`: For icons
- `lodash.clonedeep`: For utility functions
- `clsx`: For class name management
- `react-intl`: For internationalization (minimal usage)
- `react-window`: For the infinite logs panel
- `react-lottie`: For the text pre loader

## License

Licensed under the WSO2 Commercial License. 

