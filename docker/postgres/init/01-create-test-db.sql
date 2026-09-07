-- Separate database for integration tests (dev data stays in joinquest).
--
-- Suite runs no longer use this one: scripts/test-backend.sh mints a per-run
-- joinquest_test_<run id> so concurrent runs cannot share fixtures (JQ-128).
-- It stays for anyone pointing DATABASE_URL / TEST_DATABASE_URL here by hand,
-- which testdb.RequireURL still accepts.
CREATE DATABASE joinquest_test OWNER app;
