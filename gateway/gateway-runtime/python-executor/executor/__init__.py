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

"""Python Executor for WSO2 API Platform Gateway."""

from __future__ import annotations

import sys
from pathlib import Path


def _ensure_sdk_python_path() -> None:
    """Allow both source-tree and packaged-runtime imports."""
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
