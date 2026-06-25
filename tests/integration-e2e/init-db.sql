-- platform-api and gateway-controller keep separate schemas with overlapping
-- table names (artifacts, gateways, subscriptions, ...), so they must live in
-- different databases on the shared server.
CREATE DATABASE platform_api;
CREATE DATABASE gateway_test;
-- Second gateway-controller store for the multi-gateway scenario.
CREATE DATABASE gateway_test2;
