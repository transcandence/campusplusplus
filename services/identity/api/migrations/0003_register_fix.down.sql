-- Rollback: 0003_register_fix

-- Supprime l'index et la table outbox
DROP INDEX IF EXISTS idx_outbox_pending;
DROP TABLE IF EXISTS outbox_events;

-- Rétablit les contraintes NOT NULL sur email (état de la migration 0002)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_contact_ck;
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
