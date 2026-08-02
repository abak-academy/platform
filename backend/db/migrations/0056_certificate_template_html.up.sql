-- FE-serialized self-contained certificate HTML with {{token}} placeholders
-- (async redesign 2026-08-02): the admin design editor renders
-- CertificateDocument to static markup once per design save, fonts and
-- static assets embedded, and stores it here. The worker substitutes
-- verified DB values into the {{token}} spots at generation time — no
-- Go-side HTML builder, no Gotenberg fetch-back.
ALTER TABLE exam ADD COLUMN certificate_template_html TEXT;
