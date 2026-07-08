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

-- Full-text search query for dp_api_metadata (PostgreSQL only).
-- Loaded and executed by src/dao/apiMetadata.js :: searchAPIMetadata().
--
-- Named parameters (passed as Sequelize replacements):
--   :searchTerm   — the user-supplied search string
--   :orgId        — the organisation UUID to scope results to
--   :viewId       — nullable; the view UUID to scope results to (API must have a label mapped
--                   to this view). `view` is an optional query param — when omitted, results
--                   are unscoped by view rather than matching nothing.
--   :includeType  — nullable; when set, only rows with metadata.type = :includeType match
--   :excludeType  — nullable; when set, rows with metadata.type = :excludeType are excluded
--                   (keeps /apis and /mcp-servers list results type-scoped at the SQL level
--                   rather than relying on callers to filter in application code)
--
-- Other dialects use a LIKE-based fallback in searchAPIMetadataFallback().

SELECT
    metadata.*,
    COALESCE(
        JSONB_AGG(DISTINCT JSONB_BUILD_OBJECT('api_uuid', images.api_uuid, 'lookup_key', images.lookup_key, 'file_name', images.file_name, 'type', images.type))
            FILTER (WHERE images.api_uuid IS NOT NULL),
        '[]'
    ) AS "DP_API_CONTENTs",
    COALESCE(
        JSONB_AGG(DISTINCT TO_JSONB(spm.*)) FILTER (WHERE spm.api_uuid IS NOT NULL),
        '[]'
    ) AS "DP_API_SUBSCRIPTION_PLAN_MAPPING",
    COALESCE(
        ARRAY_AGG(DISTINCT lbl.name) FILTER (WHERE lbl.name IS NOT NULL),
        '{}'
    ) AS "DP_LABELs",
    COALESCE(
        ARRAY_AGG(DISTINCT tg.name) FILTER (WHERE tg.name IS NOT NULL),
        '{}'
    ) AS "DP_TAGs",
    ts_rank(
        to_tsvector('english', metadata.metadata_search::text),
        plainto_tsquery('english', COALESCE(:searchTerm, ''))
    ) AS rank_metadata,
    STRING_AGG(
        DISTINCT CASE
            WHEN content.file_content IS NOT NULL
            AND to_tsvector('english', convert_from(content.file_content, 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
            THEN content.type
            ELSE 'METADATA'
        END, ', '
    ) AS "DATA_SOURCE"
FROM
    dp_api_metadata metadata
LEFT JOIN
    dp_api_contents content
    ON metadata.uuid = content.api_uuid
    AND (
        content.file_name LIKE '%.hbs'
        OR content.file_name LIKE '%.md%'
        OR content.file_name LIKE '%.json%'
        OR content.file_name LIKE '%.xml%'
        OR content.file_name LIKE '%.graphql%'
    )
LEFT OUTER JOIN
    dp_api_contents images
    ON metadata.uuid = images.api_uuid AND images.type = 'IMAGE'
LEFT OUTER JOIN
    dp_api_subscription_plan_mappings spm
    ON metadata.uuid = spm.api_uuid
LEFT OUTER JOIN
    dp_api_label_mappings alm_join
    ON metadata.uuid = alm_join.api_uuid
LEFT OUTER JOIN
    dp_labels lbl
    ON alm_join.label_uuid = lbl.uuid
LEFT OUTER JOIN
    dp_api_tag_mappings atm_join
    ON metadata.uuid = atm_join.api_uuid
LEFT OUTER JOIN
    dp_tags tg
    ON atm_join.tag_uuid = tg.uuid
WHERE
    (
        to_tsvector('english', metadata.metadata_search::text) @@ plainto_tsquery('english', COALESCE(:searchTerm, ''))
        OR (
            content.file_content IS NOT NULL AND
            to_tsvector('english', convert_from(content.file_content, 'UTF8')) @@ plainto_tsquery('english', :searchTerm)
        )
    )
    AND metadata.org_uuid = :orgId
    AND (:includeType::text IS NULL OR metadata.type = :includeType)
    AND (:excludeType::text IS NULL OR metadata.type != :excludeType)
    AND (
        :viewId::uuid IS NULL
        OR EXISTS (
            SELECT 1
            FROM dp_api_label_mappings alm
            JOIN dp_view_label_mappings vlm ON alm.label_uuid = vlm.label_uuid
            WHERE alm.api_uuid = metadata.uuid AND vlm.view_uuid = :viewId
        )
    )
GROUP BY
    metadata.uuid
ORDER BY
    rank_metadata DESC;
