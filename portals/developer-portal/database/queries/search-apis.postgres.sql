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
        JSON_AGG(JSON_BUILD_OBJECT('API_UUID', images."API_UUID", 'LOOKUP_KEY', images."LOOKUP_KEY", 'FILE_NAME', images."FILE_NAME", 'TYPE', images."TYPE"))
            FILTER (WHERE images."API_UUID" IS NOT NULL),
        '[]'
    ) AS "DP_API_CONTENTs",
    COALESCE(
        JSON_AGG("DP_API_SUBSCRIPTION_PLAN_MAPPING") FILTER (WHERE "DP_API_SUBSCRIPTION_PLAN_MAPPING"."API_UUID" IS NOT NULL),
        '[]'
    ) AS "DP_API_SUBSCRIPTION_PLAN_MAPPING",
    COALESCE(
        ARRAY_AGG(DISTINCT "DP_LABEL"."NAME") FILTER (WHERE "DP_LABEL"."NAME" IS NOT NULL),
        '{}'
    ) AS "DP_LABELs",
    COALESCE(
        ARRAY_AGG(DISTINCT "DP_TAG"."NAME") FILTER (WHERE "DP_TAG"."NAME" IS NOT NULL),
        '{}'
    ) AS "DP_TAGs",
    ts_rank(
        to_tsvector('english', metadata."METADATA_SEARCH"::text),
        plainto_tsquery('english', COALESCE(:searchTerm, ''))
    ) AS "rank_metadata",
    STRING_AGG(
        DISTINCT CASE
            WHEN content."FILE_CONTENT" IS NOT NULL
            AND to_tsvector('english', convert_from(content."FILE_CONTENT", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
            THEN content."TYPE"
            ELSE 'METADATA'
        END, ', '
    ) AS "DATA_SOURCE"
FROM
    "DP_API_METADATA" metadata
LEFT JOIN
    "DP_API_CONTENT" content
    ON metadata."UUID" = content."API_UUID"
    AND (
        content."FILE_NAME" LIKE '%.hbs'
        OR content."FILE_NAME" LIKE '%.md%'
        OR content."FILE_NAME" LIKE '%.json%'
        OR content."FILE_NAME" LIKE '%.xml%'
        OR content."FILE_NAME" LIKE '%.graphql%'
    )
LEFT OUTER JOIN
    "DP_API_CONTENT" images
    ON metadata."UUID" = images."API_UUID" AND images."TYPE" = 'IMAGE'
LEFT OUTER JOIN
    "DP_API_SUBSCRIPTION_PLAN_MAPPING"
    ON metadata."UUID" = "DP_API_SUBSCRIPTION_PLAN_MAPPING"."API_UUID"
LEFT OUTER JOIN
    "DP_API_LABEL_MAPPING"
    ON metadata."UUID" = "DP_API_LABEL_MAPPING"."API_UUID"
LEFT OUTER JOIN
    "DP_LABEL"
    ON "DP_API_LABEL_MAPPING"."LABEL_UUID" = "DP_LABEL"."UUID"
LEFT OUTER JOIN
    "DP_API_TAG_MAPPING"
    ON metadata."UUID" = "DP_API_TAG_MAPPING"."API_UUID"
LEFT OUTER JOIN
    "DP_TAG"
    ON "DP_API_TAG_MAPPING"."TAG_UUID" = "DP_TAG"."UUID"
WHERE
    (
        to_tsvector('english', metadata."METADATA_SEARCH"::text) @@ plainto_tsquery('english', COALESCE(:searchTerm, ''))
        OR (
            content."FILE_CONTENT" IS NOT NULL AND
            to_tsvector('english', convert_from(content."FILE_CONTENT", 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
        )
    )
    AND metadata."ORG_UUID" = :orgID
GROUP BY
    metadata."UUID"
ORDER BY
    rank_metadata DESC;
