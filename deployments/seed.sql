-- Demo seed data for local development.
-- Apply with:
--   docker compose -f deployments/docker-compose.yml exec -T postgres \
--     psql -U kalia -d kalia_store < deployments/seed.sql
--
-- The admin account is seeded automatically from ADMIN_USERNAME/ADMIN_PASSWORD
-- on API startup, so it is not created here.

-- Products
INSERT INTO products (name, description, base_price, is_active, credential_schema)
VALUES
  ('Netflix Premium', '4K UHD, shared profile', 55000, TRUE,
   '[{"key":"email","label":"Email","type":"string","required":true},
     {"key":"password","label":"Password","type":"secret","required":true},
     {"key":"profile","label":"Profile","type":"string","required":false}]'::jsonb),
  ('ChatGPT Plus', '1 month, private', 120000, TRUE,
   '[{"key":"email","label":"Email","type":"string","required":true},
     {"key":"password","label":"Password","type":"secret","required":true}]'::jsonb)
ON CONFLICT DO NOTHING;

-- Inventory for Netflix (product id resolved by name)
INSERT INTO accounts (product_id, credentials, status)
SELECT p.id,
       '{"email":"netflix1@example.com","password":"changeme1","profile":"P1"}'::jsonb,
       'AVAILABLE'
FROM products p WHERE p.name = 'Netflix Premium'
ON CONFLICT DO NOTHING;

INSERT INTO accounts (product_id, credentials, status)
SELECT p.id,
       '{"email":"netflix2@example.com","password":"changeme2","profile":"P2"}'::jsonb,
       'AVAILABLE'
FROM products p WHERE p.name = 'Netflix Premium'
ON CONFLICT DO NOTHING;

-- Telegram content
INSERT INTO telegram_menus (command, title, reply_text, is_enabled, sort_order)
VALUES ('/order', 'Order', 'Choose a product to buy:', TRUE, 1)
ON CONFLICT (command) DO NOTHING;

INSERT INTO telegram_responses (command, reply_text, is_enabled)
VALUES ('/testimoni', 'See customer reviews at t.me/kalia_reviews', TRUE)
ON CONFLICT (command) DO NOTHING;
