#!/usr/bin/env python3
# Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

Starts the gRPC server on a Unix domain socket (default) or TCP port,
loads all registered Python policies, and serves ExecuteStream RPCs
from the Go Policy Engine.
"""

import argparse
import asyncio
import logging
import os
import signal
import sys

try:
    import tomllib
except ImportError:
    try:
        import tomli as tomllib
    except ImportError:
        tomllib = None

from executor.server import PythonExecutorServer


DEFAULT_LISTEN_ADDRESS = "/var/run/api-platform/python-executor.sock"


def positive_int(value):
    """Validate that the value is a positive integer."""
    try:
        ivalue = int(value)
        if ivalue <= 0:
            raise ValueError
        return ivalue
    except (ValueError, TypeError) as e:
        raise argparse.ArgumentTypeError(f"'{value}' is not a positive integer") from e


def _parse_args():
    """Parse CLI flags. Environment variables are used as defaults so that
    docker-entrypoint.sh can pass --py.* overrides that take precedence."""

    def _flag_was_provided(flag_name: str) -> bool:
        return any(
            arg == flag_name or arg.startswith(f"{flag_name}=")
            for arg in sys.argv[1:]
        )

    def _resolve_positive_int(
        flag_name: str,
        value: str | None,
        fallback: int,
    ) -> int:
        if value is None:
            return fallback
        try:
            return positive_int(value)
        except argparse.ArgumentTypeError as e:
            if _flag_was_provided(flag_name):
                parser.error(f"{flag_name}: {e}")
            # Invalid env-derived default should not block startup or valid CLI overrides.
            return fallback

    # First pass: Extract just the config file path
    pre_parser = argparse.ArgumentParser(add_help=False)
    pre_parser.add_argument("--config", default=os.environ.get("PYTHON_EXECUTOR_CONFIG"))
    pre_args, _ = pre_parser.parse_known_args()

    config_data = {}
    if pre_args.config and os.path.exists(pre_args.config):
        if tomllib is None:
            sys.exit(f"Error: tomli/tomllib not available to parse {pre_args.config}")
        try:
            with open(pre_args.config, "rb") as f:
                config_data = tomllib.load(f)
        except Exception as e:
            sys.exit(f"Error reading config {pre_args.config}: {e}")

    py_cfg = config_data.get("python_executor", {})
    srv_cfg = py_cfg.get("server", {})

    listen_default = os.environ.get("PYTHON_EXECUTOR_LISTEN")
    if listen_default is None:
        if srv_cfg:
            mode = srv_cfg.get("mode", "uds")
            if mode == "tcp":
                listen_default = f"{srv_cfg.get('host', 'localhost')}:{srv_cfg.get('port', 9010)}"
            else:
                listen_default = DEFAULT_LISTEN_ADDRESS
        else:
            listen_default = DEFAULT_LISTEN_ADDRESS

    timeout_default = os.environ.get("PYTHON_POLICY_TIMEOUT")
    if timeout_default is None and "timeout" in py_cfg:
        timeout_raw = py_cfg["timeout"]
        if isinstance(timeout_raw, str) and timeout_raw.endswith("s"):
            timeout_default = timeout_raw[:-1]
        else:
            timeout_default = str(timeout_raw)

    parser = argparse.ArgumentParser(description="Python Executor gRPC server")
    parser.add_argument(
        "--config",
        default=os.environ.get("PYTHON_EXECUTOR_CONFIG"),
        help="Path to TOML config file (env: PYTHON_EXECUTOR_CONFIG)",
    )
    parser.add_argument(
        "--listen",
        default=listen_default,
        help=f"Listen address (env: PYTHON_EXECUTOR_LISTEN, default: %(default)s)",
    )
    parser.add_argument(
        "--workers",
        default=os.environ.get("PYTHON_POLICY_WORKERS"),
        help="Number of gRPC server workers (env: PYTHON_POLICY_WORKERS)",
    )
    parser.add_argument(
        "--max-concurrent",
        default=os.environ.get("PYTHON_POLICY_MAX_CONCURRENT"),
        help="Max concurrent policy executions (env: PYTHON_POLICY_MAX_CONCURRENT)",
    )
    parser.add_argument(
        "--timeout",
        default=timeout_default,
        help="Timeout in seconds for policy execution (env: PYTHON_POLICY_TIMEOUT)",
    )
    parser.add_argument(
        "--log-level",
        default=os.environ.get("LOG_LEVEL", "info"),
        help="Log level (env: LOG_LEVEL)",
    )
    args = parser.parse_args()
    args.workers = _resolve_positive_int("--workers", args.workers, 4)
    args.max_concurrent = _resolve_positive_int(
        "--max-concurrent",
        args.max_concurrent,
        100,
    )
    args.timeout = _resolve_positive_int(
        "--timeout",
        args.timeout,
        30,
    )
    return args


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

    listen_address = args.listen
    worker_count = args.workers
    max_concurrent = args.max_concurrent
    timeout = args.timeout

    logger.info(
        "Python Executor starting (listen=%s, workers=%s, "
        "max_concurrent=%s, timeout=%ss, log_level=%s)",
        listen_address, worker_count, max_concurrent, timeout, LOG_LEVEL,
    )

    server = PythonExecutorServer(listen_address, worker_count, max_concurrent, timeout)

    # Graceful shutdown on SIGTERM/SIGINT
    loop = asyncio.get_event_loop()
    shutdown_task = None

    def signal_handler():
        nonlocal shutdown_task
        if shutdown_task is None or shutdown_task.done():
            logger.info("Received shutdown signal")
            shutdown_task = asyncio.create_task(server.shutdown())
            
            def on_shutdown_done(task):
                try:
                    task.result()
                except Exception as e:
                    logger.error(f"Error during shutdown: {e}")
                    
            shutdown_task.add_done_callback(on_shutdown_done)

    for sig in (signal.SIGTERM, signal.SIGINT):
        loop.add_signal_handler(sig, signal_handler)

    try:
        await server.start()
        await server.wait_for_termination()
    except asyncio.CancelledError:
        logger.info("Server cancelled")
    finally:
        if shutdown_task is not None:
            await shutdown_task
        else:
            await server.shutdown()


if __name__ == "__main__":
    asyncio.run(main())
