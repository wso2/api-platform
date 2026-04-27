# Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.

from __future__ import annotations

import sys
from pathlib import Path


def _ensure_sdk_python_path() -> None:
    resolved = Path(__file__).resolve()
    search_roots = [resolved.parent, *resolved.parents]

    for root in search_roots:
        for candidate in (
            root / "sdk-python" / "src",
            root / "python-executor",
            root,
        ):
            package_dir = candidate / "wso2_gateway_policy_sdk"
            if package_dir.is_dir():
                candidate_str = str(candidate)
                if candidate_str not in sys.path:
                    sys.path.insert(0, candidate_str)
                return


_ensure_sdk_python_path()
