SET NAMES utf8mb4;

-- started_at determina la posición actual de cada señal de forma determinista.
-- El desfase de 5 minutos entre ambas equivale a 30 rotaciones de diferencia
-- (30 × 10s = 300s), por lo que siempre muestran segmentos distintos.
INSERT INTO streams_metadata (title, description, segment_path, started_at, is_active) VALUES
    ('Kids4Fun', 'Big Bug Bunny 24/7', 'big_bug_bunny', '2020-01-01 00:00:00', TRUE),
    ('CineArte', 'Señal alternativa para validación de distintas señales', 'big_bug_bunny', '2020-01-01 00:05:00', TRUE);
