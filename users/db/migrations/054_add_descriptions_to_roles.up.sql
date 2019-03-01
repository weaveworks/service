ALTER TABLE roles ADD COLUMN description text NOT NULL DEFAULT '';

UPDATE roles SET description = 'Can add/remove team members, update billing info, delete and move instances' WHERE id = 'admin';
UPDATE roles SET description = 'Can deploy new image versions, change alert configurations, edit notebooks and delete pods etc' WHERE id = 'editor';
UPDATE roles SET description = 'Has a read-only view of the cluster' WHERE id = 'viewer';
