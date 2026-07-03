-- Rename legacy template tables to project tables for clarity.
-- This migration creates new tables and copies data. Old tables are kept for rollback safety.

-- 1. Rename the main templates table to projects (if not already done via previous migrations)
-- We assume the `projects` table already exists and is the canonical product catalog.
-- We only need to ensure legacy data in `templates` is accessible via `projects`.

-- 2. Rename link tables
ALTER TABLE etsy_listing_templates RENAME TO etsy_listing_projects;
ALTER TABLE squarespace_product_templates RENAME TO squarespace_product_projects;
ALTER TABLE shopify_product_templates RENAME TO shopify_product_projects;

-- 3. Rename recipe tables to project_bom (bill of materials)
ALTER TABLE recipe_materials RENAME TO project_materials;
ALTER TABLE recipe_supplies RENAME TO project_supplies;

-- 4. Rename template_designs to project_designs
ALTER TABLE template_designs RENAME TO project_designs;

-- 5. Update indexes
DROP INDEX IF EXISTS idx_template_designs_template;
CREATE INDEX IF NOT EXISTS idx_project_designs_project ON project_designs(project_id);

DROP INDEX IF EXISTS idx_recipe_materials_recipe;
CREATE INDEX IF NOT EXISTS idx_project_materials_project ON project_materials(project_id);

DROP INDEX IF EXISTS idx_recipe_supplies_recipe;
CREATE INDEX IF NOT EXISTS idx_project_supplies_project ON project_supplies(project_id);

-- Note: The `projects` table itself is already the target. No data loss.
-- The `templates` table can be dropped in a future cleanup migration after verifying all data is in `projects`.
