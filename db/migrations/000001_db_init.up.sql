CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    IF row(NEW.*) IS DISTINCT FROM row(OLD.*) THEN
      NEW.updated_at = now(); 
      RETURN NEW;
    ELSE
      RETURN OLD;
    END IF;
END;
$$ language 'plpgsql';

CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY,
    email TEXT UNIQUE,
    password BYTEA,
    full_name TEXT,
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

CREATE TABLE IF NOT EXISTS user_notes (
    id UUID PRIMARY KEY,
    title VARCHAR(255),
    content TEXT,
    user_id UUID references users(id),
    created_at timestamptz DEFAULT now(),
    updated_at timestamptz DEFAULT now()
);

CREATE TRIGGER tr_users_bu BEFORE UPDATE on users
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER tr_user_notes_bu BEFORE UPDATE on user_notes
  FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
