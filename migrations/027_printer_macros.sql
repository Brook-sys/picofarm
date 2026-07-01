-- printer_macros: user-defined G-code macros/commands
CREATE TABLE IF NOT EXISTS printer_macros (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    command TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_printer_macros_title ON printer_macros(title);
