-- Backfill project tags for STL files that were already linked to project parts.

INSERT OR IGNORE INTO tags (id, name, color, created_at, updated_at)
SELECT lower(hex(randomblob(4))) || '-' || lower(hex(randomblob(2))) || '-4' || substr(lower(hex(randomblob(2))), 2) || '-' || substr('89ab', abs(random()) % 4 + 1, 1) || substr(lower(hex(randomblob(2))), 2) || '-' || lower(hex(randomblob(6))),
       'Projeto: ' || p.name,
       '#f59e0b',
       CURRENT_TIMESTAMP,
       CURRENT_TIMESTAMP
FROM projects p
WHERE EXISTS (
  SELECT 1
  FROM parts pa
  JOIN designs d ON d.part_id = pa.id
  JOIN stl_files sf ON sf.file_id = d.file_id
  WHERE pa.project_id = p.id
    AND d.file_type = 'stl'
)
AND NOT EXISTS (
  SELECT 1 FROM tags t WHERE t.name = 'Projeto: ' || p.name
);

INSERT OR IGNORE INTO stl_file_tags (stl_file_id, tag_id)
SELECT sf.id, t.id
FROM projects p
JOIN parts pa ON pa.project_id = p.id
JOIN designs d ON d.part_id = pa.id
JOIN stl_files sf ON sf.file_id = d.file_id
JOIN tags t ON t.name = 'Projeto: ' || p.name
WHERE d.file_type = 'stl';
