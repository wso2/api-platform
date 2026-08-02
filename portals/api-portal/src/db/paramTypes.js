/*
 * Copyright (c) 2026, WSO2 LLC. (http://www.wso2.com) All Rights Reserved.
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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */
'use strict';

/**
 * Marker wrapper for a bind parameter whose column type can't be reliably
 * inferred from the JS value alone by every dialect driver — namely a
 * nullable VARBINARY(MAX)/BYTEA/BLOB column.
 *
 * `pg` and `better-sqlite3` don't need this: both bind a `null` parameter
 * without trying to resolve a concrete SQL type for it. The `mssql` package
 * (tedious-based) infers a SQL type per-parameter from the JS value passed to
 * `request.input()` when no explicit type is given — for `null` there is no
 * value to infer from, so it falls back to `NVarChar`, and SQL Server refuses
 * the implicit NVarChar -> VARBINARY conversion at execution time (unlike,
 * say, NVarChar -> INT/DATETIME2, which it tolerates).
 *
 * DAOs writing to a nullable binary column should wrap that column's bind
 * value (Buffer or null) with `binaryParam()` instead of passing it bare.
 * Each adapter's param-coercion step unwraps it: the mssql adapter binds it
 * with an explicit `VarBinary(MAX)` type; postgres/sqlite just use `.value`
 * unchanged, since neither driver has this inference gap.
 */
class BinaryParam {
    constructor(value) {
        this.value = value;
    }
}

function binaryParam(value) {
    return new BinaryParam(value);
}

function isBinaryParam(value) {
    return value instanceof BinaryParam;
}

module.exports = { BinaryParam, binaryParam, isBinaryParam };
