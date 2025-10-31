# Use the latest Node.js 22 with Debian Bookworm as the base image
FROM node:23-bookworm-slim

# Set a non-interactive frontend to prevent prompts
ENV DEBIAN_FRONTEND=noninteractive 

# Update package lists and install necessary utilities in a single RUN to reduce image layers
RUN apt-get update && apt-get install -y --no-install-recommends \
    wget \
    bash \
    coreutils \
    default-jre \
    && rm -rf /var/lib/apt/lists/*

# Verify the installed wget version
RUN wget --version

# Set the working directory inside the container
WORKDIR /app

# Copy package.json and package-lock.json first to leverage Docker caching
COPY package*.json ./
COPY openapitools.json ./

# Install dependencies using npm ci for faster, cleaner installations in production
RUN npm ci --only=production

RUN npx @openapitools/openapi-generator-cli version

# Copy the rest of the application files
COPY . .

# Expose the application port
EXPOSE 8080

# Create a non-root user with UID 10001 to satisfy Checkov CKV_CHOREO_1
RUN groupadd -g 10001 appgroup && useradd -m -u 10001 -g appgroup -s /bin/bash appuser

# Explicitly switch to UID 10001 instead of using "appuser"
USER 10001

# Start the Node.js application
CMD ["node", "src/app.js"]
