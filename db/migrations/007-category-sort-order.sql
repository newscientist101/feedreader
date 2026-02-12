-- Add sort_order column to categories for drag-and-drop reordering
ALTER TABLE categories ADD COLUMN sort_order INTEGER DEFAULT 0;

-- Initialize sort_order based on current ID order
UPDATE categories SET sort_order = id;
