\i '0-postgres-sakila-setup.sql'
\i '1-postgres-sakila-schema.sql'
\i '2-postgres-sakila-insert-data.sql'

-- Keep upstream scripts vanilla; only provide missing role expected by upstream user script.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sakila') THEN
        CREATE ROLE sakila;
    END IF;
END
$$;

\i '3-postgres-sakila-user.sql'
