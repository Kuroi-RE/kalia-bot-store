ALTER TABLE accounts DROP CONSTRAINT IF EXISTS fk_accounts_reserved_order;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS telegram_users;
