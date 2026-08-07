FROM node:22-alpine

WORKDIR /app

# Pin the exact upstream version in package.json — never `npx -y` at container
# start, so the test run does not depend on npm registry access at runtime.
COPY package.json ./
RUN npm install --omit=dev

EXPOSE 3001

CMD ["node", "node_modules/@modelcontextprotocol/server-everything/dist/index.js", "streamableHttp"]
