-- Right: same data + one more user
INSERT INTO users (name, email) VALUES ('alice', 'alice@example.com');
INSERT INTO users (name, email) VALUES ('bob', 'bob@example.com');
INSERT INTO users (name, email, role) VALUES ('admin', 'admin@example.com', 'admin');
INSERT INTO orders (user_id, total) VALUES (1, 99.50);
