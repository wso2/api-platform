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

// The forward proxy a headed browser on the host browses through (UI_HEADED): it runs on the
// block's network, so it resolves the aliases the in-network browser would. Plain HTTP is
// forwarded; HTTPS and WebSockets arrive as CONNECT and are tunnelled.
const http = require('http');
const net = require('net');

const server = http.createServer((req, res) => {
  let target;
  try {
    target = new URL(req.url);
  } catch {
    res.writeHead(400).end('absolute URL required');
    return;
  }
  const headers = { ...req.headers };
  delete headers['proxy-connection'];
  const upstream = http.request(
    {
      host: target.hostname,
      port: target.port || 80,
      method: req.method,
      path: target.pathname + target.search,
      headers,
    },
    (up) => {
      res.writeHead(up.statusCode, up.headers);
      up.pipe(res);
    },
  );
  upstream.on('error', () => {
    if (!res.headersSent) res.writeHead(502);
    res.end();
  });
  req.pipe(upstream);
});

server.on('connect', (req, client, head) => {
  let hostname, port;
  try {
    ({ hostname, port } = new URL(`http://${req.url}`));
  } catch {
    client.end('HTTP/1.1 400 Bad Request\r\n\r\n');
    return;
  }
  const upstream = net.connect(Number(port) || 443, hostname.replace(/^\[|\]$/g, ''), () => {
    client.write('HTTP/1.1 200 Connection Established\r\n\r\n');
    upstream.write(head);
    upstream.pipe(client);
    client.pipe(upstream);
  });
  const close = () => {
    client.destroy();
    upstream.destroy();
  };
  upstream.on('error', close);
  client.on('error', close);
});

server.listen(Number(process.env.PROXY_PORT) || 3128, '0.0.0.0');
