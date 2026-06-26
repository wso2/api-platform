-- --------------------------------------------------------------------
-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.
-- --------------------------------------------------------------------

-- Full-text search query for DP_API_METADATA (PostgreSQL only).
-- Loaded and executed by src/dao/apiMetadata.js :: searchAPIMetadata().
--
-- Named parameters (passed as Sequelize replacements):
--   :searchTerm  — the user-supplied search string
--   :orgID       — the organisation UUID to scope results to
--
-- Other dialects use a LIKE-based fallback in searchAPIMetadataFallback().

SELECT
    metadata.*,
    COALESCE(
        JSON_AGG(JSON_BUILD_OBJECT('API_ID', images."API_ID", 'LOOKUP_KEY', images."LOOKUP_KEY", 'FILE_NAME', images."FILE_NAME", 'TYPE', images."TYPE"))
            FILTER (WHERE images."API_ID" IS NOT NULL),
        '[]'
    ) AS "DP_API_CONTENTs",
    COALESCE(
        JSON_AGG("DP_API_SUBSCRIPTION_PLAN") FILTER (WHERE "DP_API_SUBSCRIPTION_PLAN"."API_ID" IS NOT NULL),
        '[]'
    ) AS "DP_API_SUBSCRIPTION_PLAN",
    COALESCE(
        ARRAY_AGG(DISTINCT "DP_LABEL"."NAME") FILTER (WHERE "DP_LABEL"."NAME" IS NOT NULL),
        '{}'
    ) AS "DP_LABELs",
    ts_rank(
        to_tsvector('english', metadata."METADATA_SEARCH"::text),
        plainto_tsquery('english', COALESCE(:searchTerm, ''))
    ) AS "rank_metadata",
    STRING_AGG(
        DISTINCT CASE
            WHEN content."API_FILE" IS NOT NULL
            AND to_tsvector('english', convert_from(content."API_FILE", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
            THEN content."TYPE"
            ELSE 'METADATA'
        END, ', '
    ) AS "DATA_SOURCE"
FROM
    "DP_API_METADATA" metadata
LEFT JOIN
    "DP_API_CONTENT" content
    ON metadata."ID" = content."API_ID"
    AND (
        content."FILE_NAME" LIKE '%.hbs'
        OR content."FILE_NAME" LIKE '%.md%'
        OR content."FILE_NAME" LIKE '%.json%'
        OR content."FILE_NAME" LIKE '%.xml%'
        OR content."FILE_NAME" LIKE '%.graphql%'
    )
LEFT OUTER JOIN
    "DP_API_CONTENT" images
    ON metadata."ID" = images."API_ID" AND images."TYPE" = 'IMAGE'
LEFT OUTER JOIN
    "DP_API_SUBSCRIPTION_PLAN"
    ON metadata."ID" = "DP_API_SUBSCRIPTION_PLAN"."API_ID"
LEFT OUTER JOIN
    "DP_API_LABEL"
    ON metadata."ID" = "DP_API_LABEL"."API_ID"
LEFT OUTER JOIN
    "DP_LABEL"
    ON "DP_API_LABEL"."LABEL_ID" = "DP_LABEL"."LABEL_ID"
WHERE
    (
        to_tsvector('english', metadata."METADATA_SEARCH"::text) @@ plainto_tsquery('english', COALESCE(:searchTerm, ''))
        OR (
            content."API_FILE" IS NOT NULL AND
            to_tsvector('english', convert_from(content."API_FILE", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
        )
    )
    AND metadata."ORG_ID" = :orgID
GROUP BY
    metadata."ID"
ORDER BY
    rank_metadata DESC;
