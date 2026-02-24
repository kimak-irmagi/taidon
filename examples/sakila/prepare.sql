-- Keep upstream scripts vanilla; prepare prerequisites in the wrapper only.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sakila') THEN
        CREATE ROLE sakila;
    END IF;
END
$$;

SELECT 'CREATE DATABASE sakila OWNER sakila'
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'sakila') \gexec

\connect sakila
\i '0-postgres-sakila-setup.sql'
\i '1-postgres-sakila-schema.sql'
\i '2-postgres-sakila-insert-data.sql'
\i '3-postgres-sakila-user.sql'
