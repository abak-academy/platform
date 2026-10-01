ALTER TABLE exam
    ADD COLUMN format_version INT NOT NULL DEFAULT 1 CHECK (format_version > 0),
    ADD COLUMN navigation_policy TEXT NOT NULL DEFAULT 'free' CHECK (navigation_policy IN ('free', 'sequential'));

ALTER TABLE exam_test
    ADD COLUMN question_presentation TEXT NOT NULL DEFAULT 'one_per_page' CHECK (question_presentation IN ('one_per_page', 'all_on_page'));

ALTER TABLE test
    ADD COLUMN shared_material_html TEXT,
    ADD COLUMN instructions_html TEXT;

ALTER TABLE question
    ADD COLUMN material_html TEXT;
