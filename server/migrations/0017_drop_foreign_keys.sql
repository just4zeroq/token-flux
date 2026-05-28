-- +goose Up
DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT tc.table_name, tc.constraint_name
        FROM information_schema.table_constraints tc
        WHERE tc.constraint_type = 'FOREIGN KEY'
          AND tc.table_schema = 'public'
    LOOP
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I', r.table_name, r.constraint_name);
    END LOOP;
END $$;

-- +goose Down
-- Intentionally not restoring. FK constraints are dropped permanently.
