-- Safe compatibility migration: keep legacy template/recipe data, but prefer projects.

-- Orders: copy legacy template_id into project_id when the referenced template has a project successor.
UPDATE order_items
SET project_id = (
    SELECT p.id
    FROM projects p
    WHERE p.template_id = order_items.template_id
    ORDER BY p.created_at DESC
    LIMIT 1
)
WHERE project_id IS NULL
  AND template_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.template_id = order_items.template_id);

-- If IDs already point to an existing project because an integration used project IDs in a legacy template_id column, copy directly.
UPDATE order_items
SET project_id = template_id
WHERE project_id IS NULL
  AND template_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = order_items.template_id);

-- Print jobs: copy legacy recipe_id into project_id using project template lineage.
UPDATE print_jobs
SET project_id = (
    SELECT p.id
    FROM projects p
    WHERE p.template_id = print_jobs.recipe_id
    ORDER BY p.created_at DESC
    LIMIT 1
)
WHERE project_id IS NULL
  AND recipe_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.template_id = print_jobs.recipe_id);

-- Direct copy when recipe_id already stores a project ID.
UPDATE print_jobs
SET project_id = recipe_id
WHERE project_id IS NULL
  AND recipe_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = print_jobs.recipe_id);

-- Queue items: direct legacy template_id to project_id compatibility.
UPDATE queue_items
SET project_id = template_id
WHERE project_id IS NULL
  AND template_id IS NOT NULL
  AND EXISTS (SELECT 1 FROM projects p WHERE p.id = queue_items.template_id);
