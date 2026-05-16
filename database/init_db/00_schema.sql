CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS streams_metadata (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(100) NOT NULL,
    description VARCHAR(255),
    segment_path VARCHAR(255) NOT NULL,
    initial_offset INT DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE
);

CREATE TABLE IF NOT EXISTS stream_segments (
    id INT AUTO_INCREMENT PRIMARY KEY,
    stream_id INT NOT NULL,
    name VARCHAR(255) NOT NULL,
    duration DECIMAL(10,6) NOT NULL,
    position INT NOT NULL,
    FOREIGN KEY (stream_id) REFERENCES streams_metadata(id) ON DELETE CASCADE,
    UNIQUE KEY uq_stream_position (stream_id, position),
    INDEX idx_stream_position (stream_id, position)
);
