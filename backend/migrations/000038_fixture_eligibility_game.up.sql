-- Fixture game for mode-level eligibility (JQ-11/12/13/16): demonstrates all four gate
-- shapes (boolean, single-counter, boolean-as-leaf, compound AND) against a local
-- fixture HTTP server (backend/cmd/fixturegame), since no real reference game implements
-- the mode-eligibility contract yet.

INSERT INTO games (
    id, name, slug, api_base_url, icon_url, hero_url, short_description,
    category, status, visibility, tags
)
VALUES (
    'b1000000-0000-4000-8000-000000000001',
    'Eligibility Fixture',
    'eligibility-fixture',
    'http://localhost:9400',
    '/games/eligibility-fixture/icon.png',
    '/games/eligibility-fixture/hero.jpg',
    'Local fixture game demonstrating mode-level eligibility gates.',
    'catalog',
    'active',
    'public',
    '{}'
)
ON CONFLICT (id) DO NOTHING;

INSERT INTO game_modes (id, game_id, mode_key, display_name, min_players, max_players, status)
VALUES
    ('b2000000-0000-4000-8000-000000000001', 'b1000000-0000-4000-8000-000000000001', 'arena', 'Arena', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000002', 'b1000000-0000-4000-8000-000000000001', 'legendary', 'Legendary', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000003', 'b1000000-0000-4000-8000-000000000001', 'standard', 'Standard', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000004', 'b1000000-0000-4000-8000-000000000001', 'commander', 'Commander', 2, 2, 'active'),
    ('b2000000-0000-4000-8000-000000000005', 'b1000000-0000-4000-8000-000000000001', 'deck-builder', 'Deck Builder', 1, 1, 'active')
ON CONFLICT (game_id, mode_key) DO NOTHING;

INSERT INTO game_mode_seats (mode_id, seat_key, sort_order)
VALUES
    ('b2000000-0000-4000-8000-000000000001', '1', 0),
    ('b2000000-0000-4000-8000-000000000001', '2', 1),
    ('b2000000-0000-4000-8000-000000000002', '1', 0),
    ('b2000000-0000-4000-8000-000000000002', '2', 1),
    ('b2000000-0000-4000-8000-000000000003', '1', 0),
    ('b2000000-0000-4000-8000-000000000003', '2', 1),
    ('b2000000-0000-4000-8000-000000000004', '1', 0),
    ('b2000000-0000-4000-8000-000000000004', '2', 1),
    ('b2000000-0000-4000-8000-000000000005', '1', 0)
ON CONFLICT (mode_id, seat_key) DO NOTHING;
