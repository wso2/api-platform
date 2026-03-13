#!/usr/bin/env python3
# Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Python Executor entry point.

Starts the gRPC server on UDS, loads all registered Python policies,
and serves ExecuteStream RPCs from the Go Policy Engine.
"""

import argparse
import asyncio
import logging
import os
import signal
import sys

from executor.server import PythonExecutorServer


def _parse_args():
    """Parse CLI flags. Environment variables are used as defaults so that
    docker-entrypoint.sh can pass --py.* overrides that take precedence."""
    parser = argparse.ArgumentParser(description="Python Executor gRPC server")
    parser.add_argument(
        "--socket",
        default=os.environ.get("PYTHON_EXECUTOR_SOCKET", "/var/run/api-platform/python-executor.sock"),
        help="Path to the UDS socket (env: PYTHON_EXECUTOR_SOCKET)",
    )
    parser.add_argument(
        "--workers",
        type=int,
        default=int(os.environ.get("PYTHON_POLICY_WORKERS", "4")),
        help="Number of gRPC server workers (env: PYTHON_POLICY_WORKERS)",
    )
    parser.add_argument(
        "--max-concurrent",
        type=int,
        default=int(os.environ.get("PYTHON_POLICY_MAX_CONCURRENT", "100")),
        help="Max concurrent policy executions (env: PYTHON_POLICY_MAX_CONCURRENT)",
    )
    parser.add_argument(
        "--log-level",
        default=os.environ.get("LOG_LEVEL", "info"),
        help="Log level (env: LOG_LEVEL)",
    )
    return parser.parse_args()


SOCKET_PATH = os.environ.get(
    "PYTHON_EXECUTOR_SOCKET",
    "/var/run/api-platform/python-executor.sock"
)
WORKER_COUNT = int(os.environ.get("PYTHON_POLICY_WORKERS", "4"))
MAX_CONCURRENT = int(os.environ.get("PYTHON_POLICY_MAX_CONCURRENT", "100"))
LOG_LEVEL = os.environ.get("LOG_LEVEL", "info").upper()


def setup_logging():
    """Configure structured logging."""
    level = getattr(logging, LOG_LEVEL, logging.INFO)

    # Use JSON format for production, text for development
    handler = logging.StreamHandler(sys.stdout)
    handler.setLevel(level)

    # Simple format with [pye] prefix for the entrypoint to identify
    formatter = logging.Formatter(
        fmt='%(asctime)s [%(levelname)s] %(name)s: %(message)s',
        datefmt='%Y-%m-%d %H:%M:%S'
    )
    handler.setFormatter(formatter)

    # Configure root logger
    root_logger = logging.getLogger()
    root_logger.setLevel(level)
    root_logger.addHandler(handler)

    # Set specific levels for noisy loggers
    logging.getLogger("grpc").setLevel(logging.WARNING)


async def main():
    """Main entry point."""
    args = _parse_args()

    global LOG_LEVEL
    LOG_LEVEL = args.log_level.upper()

    setup_logging()
    logger = logging.getLogger(__name__)

    socket_path = args.socket
    worker_count = args.workers
    max_concurrent = args.max_concurrent

    logger.info(f"Python Executor starting (workers={worker_count}, max_concurrent={max_concurrent}, log_level={LOG_LEVEL})")

    server = PythonExecutorServer(socket_path, worker_count, max_concurrent)

    # Graceful shutdown on SIGTERM/SIGINT
    loop = asyncio.get_event_loop()

    def signal_handler():
        logger.info("Received shutdown signal")
        asyncio.create_task(server.shutdown())

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, signal_handler)

    try:
        await server.start()
        await server.wait_for_termination()
    except asyncio.CancelledError:
        logger.info("Server cancelled")
    finally:
        await server.shutdown()


if __name__ == "__main__":
    asyncio.run(main())
