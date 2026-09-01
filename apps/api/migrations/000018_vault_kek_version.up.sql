-- The master key version that wrapped each vault's data key, so the master key can be rotated
-- without re-encrypting everything at once. Existing data keys were wrapped with version 1.
ALTER TABLE vaults ADD COLUMN kek_version INTEGER NOT NULL DEFAULT 1 CHECK (kek_version > 0);
