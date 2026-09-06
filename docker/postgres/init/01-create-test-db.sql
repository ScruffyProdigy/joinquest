-- Separate database for integration tests (dev data stays in playhub).
--
-- Suite runs no longer use this one: scripts/test-backend.sh mints a per-run
-- playhub_test_<run id> so concurrent runs cannot share fixtures (JQ-128).
-- It stays for anyone pointing DATABASE_URL / TEST_DATABASE_URL here by hand,
-- which testdb.RequireURL still accepts.
CREATE DATABASE playhub_test OWNER app;
