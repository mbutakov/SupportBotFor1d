CREATE TABLE IF NOT EXISTS ticket_messages (
        id SERIAL PRIMARY KEY,
        ticket_id INTEGER NOT NULL REFERENCES tickets(id),
        sender_type TEXT NOT NULL, -- 'user' или 'support'
        sender_id BIGINT NOT NULL,
        message TEXT NOT NULL,
        created_at TIMESTAMPTZ NOT NULL
    )

    CREATE TABLE IF NOT EXISTS tickets (
        id SERIAL PRIMARY KEY,
        user_id BIGINT NOT NULL REFERENCES users(id),
        title TEXT NOT NULL,
        description TEXT NOT NULL,
        status TEXT NOT NULL,
        category TEXT NOT NULL DEFAULT 'спросить',
        created_at TIMESTAMPTZ NOT NULL,
        closed_at TIMESTAMPTZ
    )

    CREATE TABLE IF NOT EXISTS users (
        id BIGINT PRIMARY KEY,
        full_name TEXT,
        phone TEXT,
        location_lat DOUBLE PRECISION,
        location_lng DOUBLE PRECISION,
        birth_date DATE,
        is_registered BOOLEAN NOT NULL DEFAULT FALSE,
        registered_at TIMESTAMPTZ
    )

    -- Таблица для хранения информации о фотографиях в тикетах
    CREATE TABLE IF NOT EXISTS ticket_photos (
        id SERIAL PRIMARY KEY,
        ticket_id INTEGER NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
        sender_type VARCHAR(10) NOT NULL CHECK (sender_type IN ('user', 'support')),
        sender_id BIGINT NOT NULL,
        file_path VARCHAR(255) NOT NULL,
        file_id VARCHAR(255) NOT NULL,
        message_id INTEGER REFERENCES ticket_messages(id),
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
    );

    CREATE INDEX IF NOT EXISTS idx_ticket_photos_ticket_id ON ticket_photos(ticket_id);

    -- Add critical indexes for performance
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_is_registered ON users(is_registered);
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tickets_user_id_status ON tickets(user_id, status);
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tickets_status ON tickets(status);
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ticket_messages_ticket_id_created ON ticket_messages(ticket_id, created_at);
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ticket_photos_ticket_id_created ON ticket_photos(ticket_id, created_at);
    
    -- Add partial index for active tickets only
    CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_tickets_active ON tickets(user_id, created_at) WHERE status != 'closed';