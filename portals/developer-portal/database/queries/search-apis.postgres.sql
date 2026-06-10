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
        JSON_AGG("DP_API_IMAGEDATA") FILTER (WHERE "DP_API_IMAGEDATA"."API_ID" IS NOT NULL),
        '[]'
    ) AS "DP_API_IMAGEDATA",
    COALESCE(
        JSON_AGG("DP_API_SUBSCRIPTION_POLICY") FILTER (WHERE "DP_API_SUBSCRIPTION_POLICY"."API_ID" IS NOT NULL),
        '[]'
    ) AS "DP_API_SUBSCRIPTION_POLICY",
    COALESCE(
        ARRAY_AGG(DISTINCT "DP_LABELS"."NAME") FILTER (WHERE "DP_LABELS"."NAME" IS NOT NULL),
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
    ON metadata."API_ID" = content."API_ID"
    AND (
        content."FILE_NAME" LIKE '%.hbs'
        OR content."FILE_NAME" LIKE '%.md%'
        OR content."FILE_NAME" LIKE '%.json%'
        OR content."FILE_NAME" LIKE '%.xml%'
        OR content."FILE_NAME" LIKE '%.graphql%'
    )
LEFT OUTER JOIN
    "DP_API_IMAGEDATA"
    ON metadata."API_ID" = "DP_API_IMAGEDATA"."API_ID"
LEFT OUTER JOIN
    "DP_API_SUBSCRIPTION_POLICY"
    ON metadata."API_ID" = "DP_API_SUBSCRIPTION_POLICY"."API_ID"
LEFT OUTER JOIN
    "DP_API_LABELS"
    ON metadata."API_ID" = "DP_API_LABELS"."API_ID"
LEFT OUTER JOIN
    "DP_LABELS"
    ON "DP_API_LABELS"."LABEL_ID" = "DP_LABELS"."LABEL_ID"
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
    metadata."API_ID"
ORDER BY
    rank_metadata DESC;
