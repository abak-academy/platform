-- Certificate numbers are now composed in Go (FR-25); no global sequence is consumed.
DROP SEQUENCE IF EXISTS certificate_number_seq;
