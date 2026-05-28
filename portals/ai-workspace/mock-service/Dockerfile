# Minimal Dockerfile for AI Workspace mock server
FROM python:3.11-slim

# Prevent Python from buffering stdout/stderr (helpful for logs)
ENV PYTHONUNBUFFERED=1

WORKDIR /app

# Copy only what we need (the server and requirements) to keep image small
COPY . /app

# Install system deps needed by some Python packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    libssl-dev \
    libffi-dev \
  && rm -rf /var/lib/apt/lists/*

# Upgrade pip and install required Python packages
RUN pip install --no-cache-dir --upgrade pip \
  && pip install --no-cache-dir flask flask-cors pyopenssl cryptography requests PyJWT

# Expose the port the mock server uses
EXPOSE 8080

# Create a non-root user to satisfy CKV_DOCKER_3 and run the container as that user
RUN groupadd -g 10014 appgroup || true \
 && useradd -u 10014 -g appgroup -s /sbin/nologin -M appuser || true \
 && chown -R 10014:10014 /app

# Switch to the non-root user
USER 10014

# Run the mock server
CMD ["python3", "mock_server.py"]
