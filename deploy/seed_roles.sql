-- Role test accounts. Safe to run more than once.
-- Every account below signs in with the password: Role-test-1
-- The admin account is created by the server on first start (password change-me-now in Docker).

INSERT INTO users (id, username, email, password_hash, full_name, role)
SELECT gen_random_uuid(), v.username, v.email, v.password_hash, v.full_name, v.role
FROM (
    VALUES
        ('viewer1', 'viewer@localhost', '$2a$12$bs7qz5Y9yeNaeYNBeQ3GpenabHG0NjsmWBdv9GCZQBP9di5qj5KKK', 'Vera Viewer', 'viewer'),
        ('operator1', 'operator@localhost', '$2a$12$bs7qz5Y9yeNaeYNBeQ3GpenabHG0NjsmWBdv9GCZQBP9di5qj5KKK', 'Omar Operator', 'operator'),
        ('inspector1', 'inspector@localhost', '$2a$12$bs7qz5Y9yeNaeYNBeQ3GpenabHG0NjsmWBdv9GCZQBP9di5qj5KKK', 'Iris Inspector', 'inspector'),
        ('supervisor1', 'supervisor@localhost', '$2a$12$bs7qz5Y9yeNaeYNBeQ3GpenabHG0NjsmWBdv9GCZQBP9di5qj5KKK', 'Sam Supervisor', 'supervisor'),
        ('quality1', 'quality@localhost', '$2a$12$bs7qz5Y9yeNaeYNBeQ3GpenabHG0NjsmWBdv9GCZQBP9di5qj5KKK', 'Quinn Quality', 'quality_manager')
) AS v(username, email, password_hash, full_name, role)
WHERE NOT EXISTS (
    SELECT 1 FROM users u WHERE lower(u.username) = lower(v.username)
);
