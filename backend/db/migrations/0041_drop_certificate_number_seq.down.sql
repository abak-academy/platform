CREATE SEQUENCE IF NOT EXISTS certificate_number_seq;

-- Restore the counter's high-water mark. A rollback puts back the code that
-- allocates certificate numbers with nextval(), so a sequence starting over at 1
-- would re-issue numbers already in exam_session.certificate_number and every
-- allocation would fail on idx_exam_session_certificate_number. Only the legacy
-- 3-segment numbers (ABK/YYYY/NNNNNN) can collide — the 4-segment ones the
-- post-0041 code composes are not drawn from the sequence.
SELECT setval(
    'certificate_number_seq',
    COALESCE(
        (SELECT MAX(split_part(certificate_number, '/', 3)::BIGINT)
         FROM exam_session
         WHERE certificate_number ~ '^ABK/[0-9]{4}/[0-9]+$'),
        0
    ) + 1,
    false
);
