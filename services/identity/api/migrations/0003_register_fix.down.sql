-- Rollback: 0003_register_fix
--
-- 0003 made email and phone_number nullable and required at least one of
-- them. The previous schema required both. A rollback is therefore only
-- possible when every row already has both values; otherwise we fail loudly
-- instead of deleting or silently corrupting users.

-- Supprime l'index et la table outbox
DROP INDEX IF EXISTS idx_outbox_pending;
DROP TABLE IF EXISTS outbox_events;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_contact_ck;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
          FROM users
         WHERE email IS NULL
            OR phone_number IS NULL
    ) THEN
        RAISE EXCEPTION
            'Cannot roll back 0003: some users have NULL email or phone_number. Backfill both columns (or remove those users) before running this rollback.';
    END IF;
END
$$;

-- Rétablit les contraintes NOT NULL sur email et téléphone (état 0002)
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users ALTER COLUMN phone_number SET NOT NULL;
