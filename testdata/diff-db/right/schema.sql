-- Right (new): schema v2 — added role column
CREATE TABLE IF NOT EXISTS users (
  id    SERIAL PRIMARY KEY,
  name  TEXT NOT NULL,
  email TEXT,
  role  TEXT DEFAULT 'user'
);

CREATE TABLE IF NOT EXISTS orders (
  id      SERIAL PRIMARY KEY,
  user_id INT REFERENCES users(id),
  total   NUMERIC(10,2) DEFAULT 0
);

CREATE TABLE IF NOT EXISTS audit_log (
  id         SERIAL PRIMARY KEY,
  table_name TEXT,
  changed_at TIMESTAMPTZ DEFAULT now()
);
