-- Mutual TLS columns on certificates. See the sqlite file for column meanings.
-- Replaces the ALTERs appended to gateway-controller-db.sqlserver.sql (R11-NO-ALTER).
IF COL_LENGTH('dbo.certificates', 'usage') IS NULL
ALTER TABLE dbo.certificates ADD usage NVARCHAR(20) NOT NULL DEFAULT 'upstream';
IF COL_LENGTH('dbo.certificates', 'role') IS NULL
ALTER TABLE dbo.certificates ADD role NVARCHAR(20) NOT NULL DEFAULT 'client';
IF COL_LENGTH('dbo.certificates', 'match_json') IS NULL
ALTER TABLE dbo.certificates ADD match_json NVARCHAR(MAX) NULL;
IF COL_LENGTH('dbo.certificates', 'private_key_ciphertext') IS NULL
ALTER TABLE dbo.certificates ADD private_key_ciphertext NVARCHAR(MAX) NULL;
IF COL_LENGTH('dbo.certificates', 'key_algorithm') IS NULL
ALTER TABLE dbo.certificates ADD key_algorithm NVARCHAR(32) NULL;
