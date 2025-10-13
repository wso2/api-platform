const express = require('express');
const axios = require('axios');
const yaml = require('js-yaml');
const fs = require('fs');
const path = require('path');
const https = require('https');

const app = express();
const PORT = 3000;

// Allow self-signed certificates for local development
const httpsAgent = new https.Agent({
  rejectUnauthorized: false
});

// Load registration.yaml
let config = null;
const configPath = path.join(__dirname, '..', 'registration.yaml');

function loadConfig() {
  try {
    const fileContents = fs.readFileSync(configPath, 'utf8');
    config = yaml.load(fileContents);
    console.log('✓ Loaded registration.yaml');
    return true;
  } catch (e) {
    console.error('✗ Failed to load registration.yaml:', e.message);
    console.error('  Make sure you run ./kickstart.sh first!');
    return false;
  }
}

// Decode JWT without verification (for demo purposes)
function decodeJWT(token) {
  try {
    const parts = token.split('.');
    if (parts.length !== 3) {
      return null;
    }

    const payload = parts[1];
    const decoded = JSON.parse(Buffer.from(payload, 'base64').toString('utf8'));
    return decoded;
  } catch (e) {
    console.error('Failed to decode JWT:', e.message);
    return null;
  }
}

// Format JSON for display
function formatJSON(obj) {
  return JSON.stringify(obj, null, 2);
}

// Home page - redirects to OAuth authorization endpoint
app.get('/', (req, res) => {
  if (!config) {
    return res.send(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>STS Sample App - Configuration Error</title>
        <style>
          body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
          }
          .error-box {
            background: #fff3cd;
            border: 2px solid #ffc107;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
          }
          h1 { color: #333; }
          code {
            background: #f4f4f4;
            padding: 2px 6px;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
          }
        </style>
      </head>
      <body>
        <h1>⚠️ Configuration Not Found</h1>
        <div class="error-box">
          <p><strong>registration.yaml not found!</strong></p>
          <p>Please run the kickstart script first:</p>
          <pre><code>cd /home/malintha/wso2apim/gitworkspace/api-platform/sts
./kickstart.sh</code></pre>
          <p>Then restart this application.</p>
        </div>
      </body>
      </html>
    `);
  }

  // Redirect to OAuth authorization endpoint
  res.redirect(config.example_auth_url);
});

// OAuth callback handler
app.get('/callback', async (req, res) => {
  const code = req.query.code;
  const state = req.query.state;
  const error = req.query.error;

  // Handle OAuth errors
  if (error) {
    return res.send(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>OAuth Error</title>
        <style>
          body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
          }
          .error-box {
            background: #f8d7da;
            border: 2px solid #f5c6cb;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
          }
          .back-link {
            display: inline-block;
            margin-top: 20px;
            color: #667eea;
            text-decoration: none;
            font-weight: bold;
          }
        </style>
      </head>
      <body>
        <h1>❌ OAuth Error</h1>
        <div class="error-box">
          <p><strong>Error:</strong> ${error}</p>
          <p><strong>Description:</strong> ${req.query.error_description || 'No description provided'}</p>
        </div>
        <a href="/" class="back-link">← Try Again</a>
      </body>
      </html>
    `);
  }

  if (!code) {
    return res.send(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Missing Code</title>
        <style>
          body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
          }
          .back-link {
            display: inline-block;
            margin-top: 20px;
            color: #667eea;
            text-decoration: none;
            font-weight: bold;
          }
        </style>
      </head>
      <body>
        <h1>⚠️ No Authorization Code</h1>
        <p>No authorization code was received from the OAuth provider.</p>
        <a href="/" class="back-link">← Try Again</a>
      </body>
      </html>
    `);
  }

  // Build the curl command for display
  const tokenUrl = config.oauth_endpoints.token;
  const clientId = config.application.client_id;
  const clientSecret = config.application.client_secret;
  const redirectUri = config.application.redirect_uris[0];

  const curlCommand = `curl -k -X POST ${tokenUrl} \\
  -u ${clientId}:${clientSecret} \\
  -d "grant_type=authorization_code" \\
  -d "code=${code}" \\
  -d "redirect_uri=${redirectUri}"`;

  try {
    // Exchange code for token
    const tokenResponse = await axios.post(
      tokenUrl,
      new URLSearchParams({
        grant_type: 'authorization_code',
        code: code,
        redirect_uri: redirectUri
      }),
      {
        auth: {
          username: clientId,
          password: clientSecret
        },
        httpsAgent: httpsAgent,
        headers: {
          'Content-Type': 'application/x-www-form-urlencoded'
        }
      }
    );

    const tokenData = tokenResponse.data;
    const accessToken = tokenData.access_token;
    const refreshToken = tokenData.refresh_token;
    const expiresIn = tokenData.expires_in;

    // Decode JWT
    const decodedToken = decodeJWT(accessToken);

    // Build HTML response
    res.send(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>OAuth Success - STS Sample App</title>
        <style>
          body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 1200px;
            margin: 30px auto;
            padding: 20px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
          }
          .container {
            background: white;
            border-radius: 12px;
            padding: 40px;
            box-shadow: 0 10px 40px rgba(0,0,0,0.3);
          }
          h1 {
            color: #28a745;
            margin-bottom: 10px;
          }
          h2 {
            color: #333;
            border-bottom: 2px solid #667eea;
            padding-bottom: 10px;
            margin-top: 30px;
          }
          .section {
            background: #f8f9fa;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
          }
          pre {
            background: #282c34;
            color: #abb2bf;
            padding: 20px;
            border-radius: 8px;
            overflow-x: auto;
            font-family: 'Courier New', monospace;
            font-size: 13px;
            line-height: 1.5;
          }
          .token {
            word-break: break-all;
            font-family: 'Courier New', monospace;
            font-size: 12px;
            background: #fff3cd;
            padding: 15px;
            border-radius: 8px;
            border-left: 4px solid #ffc107;
          }
          .claims {
            background: #d4edda;
            border-left: 4px solid #28a745;
            padding: 15px;
            border-radius: 8px;
          }
          .claims pre {
            background: #ffffff;
            color: #333;
            margin: 10px 0 0 0;
          }
          .info-badge {
            display: inline-block;
            background: #667eea;
            color: white;
            padding: 5px 10px;
            border-radius: 4px;
            font-size: 12px;
            margin-right: 10px;
          }
          .back-link {
            display: inline-block;
            margin-top: 30px;
            color: #667eea;
            text-decoration: none;
            font-weight: bold;
          }
          .back-link:hover {
            text-decoration: underline;
          }
          table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 10px;
          }
          th, td {
            padding: 10px;
            text-align: left;
            border-bottom: 1px solid #ddd;
          }
          th {
            background: #667eea;
            color: white;
          }
          tr:hover {
            background: #f5f5f5;
          }
        </style>
      </head>
      <body>
        <div class="container">
          <h1>✅ OAuth Flow Completed Successfully!</h1>
          <p style="color: #666;">Authorization code was successfully exchanged for an access token.</p>

          <h2>📋 Token Exchange Details</h2>
          <div class="section">
            <span class="info-badge">State: ${state || 'N/A'}</span>
            <span class="info-badge">Expires in: ${expiresIn} seconds</span>
            <span class="info-badge">Token Type: Bearer</span>
          </div>

          <h2>🔧 Curl Command Executed</h2>
          <div class="section">
            <p>The following curl command was executed to exchange the authorization code for an access token:</p>
            <pre>${curlCommand}</pre>
          </div>

          <h2>🎟️ Access Token</h2>
          <div class="token">
            ${accessToken}
          </div>

          ${refreshToken ? `
            <h2>🔄 Refresh Token</h2>
            <div class="token">
              ${refreshToken}
            </div>
          ` : ''}

          <h2>🔍 Decoded JWT Claims</h2>
          <div class="claims">
            <p><strong>Access token contains the following claims:</strong></p>
            <table>
              <thead>
                <tr>
                  <th>Claim</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                ${Object.entries(decodedToken || {}).map(([key, value]) => `
                  <tr>
                    <td><strong>${key}</strong></td>
                    <td>${typeof value === 'object' ? JSON.stringify(value) : value}</td>
                  </tr>
                `).join('')}
              </tbody>
            </table>
            <p style="margin-top: 15px;"><strong>Full JWT Payload:</strong></p>
            <pre>${formatJSON(decodedToken)}</pre>
          </div>

          <h2>📦 Full Token Response</h2>
          <div class="section">
            <pre>${formatJSON(tokenData)}</pre>
          </div>

          <a href="/" class="back-link">← Start New OAuth Flow</a>
        </div>
      </body>
      </html>
    `);

  } catch (error) {
    console.error('Token exchange failed:', error.message);

    let errorDetails = error.message;
    if (error.response) {
      errorDetails = formatJSON(error.response.data);
    }

    res.send(`
      <!DOCTYPE html>
      <html>
      <head>
        <title>Token Exchange Failed</title>
        <style>
          body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            max-width: 900px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
          }
          h1 { color: #dc3545; }
          .error-box {
            background: #f8d7da;
            border: 2px solid #f5c6cb;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
          }
          pre {
            background: #282c34;
            color: #abb2bf;
            padding: 15px;
            border-radius: 8px;
            overflow-x: auto;
          }
          .back-link {
            display: inline-block;
            margin-top: 20px;
            color: #667eea;
            text-decoration: none;
            font-weight: bold;
          }
        </style>
      </head>
      <body>
        <h1>❌ Token Exchange Failed</h1>
        <div class="error-box">
          <h3>Error Details:</h3>
          <pre>${errorDetails}</pre>
        </div>
        <h3>Curl Command Attempted:</h3>
        <pre>${curlCommand}</pre>
        <a href="/" class="back-link">← Try Again</a>
      </body>
      </html>
    `);
  }
});

// Setup HTTPS options
const keyPath = path.join(__dirname, 'server.key');
const certPath = path.join(__dirname, 'server.cert');
const httpsOptions = {
  key: fs.readFileSync(keyPath),
  cert: fs.readFileSync(certPath)
};

// Start HTTPS server
const server = https.createServer(httpsOptions, app).listen(PORT, () => {
  console.log('');
  console.log('========================================');
  console.log('  STS Sample OAuth2 Application');
  console.log('========================================');
  console.log('');

  if (loadConfig()) {
    console.log(`✓ Server running at https://localhost:${PORT}`);
    console.log('');
    console.log('Next steps:');
    console.log(`  1. Open https://localhost:${PORT} in your browser`);
    console.log('  2. You will be redirected to STS login page');
    console.log(`  3. Login with username: ${config.user.username}`);
    console.log('  4. View the access token and JWT claims');
    console.log('');
  } else {
    console.log(`⚠ Server running at https://localhost:${PORT}`);
    console.log('  But registration.yaml is missing!');
    console.log('');
    console.log('  Run ./kickstart.sh first, then restart this app.');
    console.log('');
  }
});

// Graceful shutdown
process.on('SIGINT', () => {
  console.log('\n\nShutting down gracefully...');
  server.close(() => {
    console.log('Server closed');
    process.exit(0);
  });
});
