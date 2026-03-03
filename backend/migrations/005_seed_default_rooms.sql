INSERT INTO rooms (name, description)
VALUES
    ('General', 'General chat for everyone'),
    ('Random',  'Off-topic and random conversations')
ON CONFLICT (name) DO NOTHING;
