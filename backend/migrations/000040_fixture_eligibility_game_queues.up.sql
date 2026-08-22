-- The fixture eligibility game (000038) seeded game_modes but no mode_queues,
-- so ListCatalogGames' active-queue join excluded it from the games list
-- entirely. Seed a default active queue per fixture mode to fix that.

INSERT INTO mode_queues (id, mode_id, name, players_to_start, status, is_default)
VALUES
    ('b3000000-0000-4000-8000-000000000001', 'b2000000-0000-4000-8000-000000000001', 'Default', 2, 'active', true),
    ('b3000000-0000-4000-8000-000000000002', 'b2000000-0000-4000-8000-000000000002', 'Default', 2, 'active', true),
    ('b3000000-0000-4000-8000-000000000003', 'b2000000-0000-4000-8000-000000000003', 'Default', 2, 'active', true),
    ('b3000000-0000-4000-8000-000000000004', 'b2000000-0000-4000-8000-000000000004', 'Default', 2, 'active', true),
    ('b3000000-0000-4000-8000-000000000005', 'b2000000-0000-4000-8000-000000000005', 'Default', 1, 'active', true)
ON CONFLICT (mode_id, name) DO UPDATE SET
    players_to_start = EXCLUDED.players_to_start,
    status = 'active',
    is_default = true,
    updated_at = NOW();
