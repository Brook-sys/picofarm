ALTER TABLE projects ADD COLUMN source_url TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN source_provider TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN source_author TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN source_license TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN source_description TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN cover_file_id TEXT REFERENCES files(id);
