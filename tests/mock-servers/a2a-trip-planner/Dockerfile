FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

# Pin the exact SDK version in requirements.txt — never install at container
# start, so a test run does not depend on PyPI being reachable at runtime.
COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY config.py agent.py version_query.py main.py ./

EXPOSE 9099

RUN useradd --create-home --uid 10001 agent
USER agent

CMD ["python", "/app/main.py"]
