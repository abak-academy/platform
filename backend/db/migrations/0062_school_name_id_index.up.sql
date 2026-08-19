-- ListSchoolsAdmin now paginates on the same (name, id) keyset it orders by
-- (see school-bulk-list-pagination backlog: the previous WHERE id > $cursor
-- with ORDER BY name skipped/duplicated bulk-imported rows). This index
-- makes that keyset scan/seek efficient instead of a full sort.
CREATE INDEX idx_school_name_id ON school (name, id);
