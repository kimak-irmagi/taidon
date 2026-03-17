-- Right: new file — audit trigger
CREATE OR REPLACE FUNCTION audit_trigger_fn()
RETURNS TRIGGER AS $$
BEGIN
  INSERT INTO audit_log (table_name) VALUES (TG_TABLE_NAME);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
