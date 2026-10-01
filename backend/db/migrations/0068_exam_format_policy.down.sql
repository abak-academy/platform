ALTER TABLE question DROP COLUMN material_html;

ALTER TABLE test
    DROP COLUMN shared_material_html,
    DROP COLUMN instructions_html;

ALTER TABLE exam_test DROP COLUMN question_presentation;

ALTER TABLE exam
    DROP COLUMN navigation_policy,
    DROP COLUMN format_version;
