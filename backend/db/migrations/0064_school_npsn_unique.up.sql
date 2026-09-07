UPDATE school
SET npsn = NULL
WHERE npsn IS NOT NULL
  AND (
    npsn !~ '[^[:space:]]'
    OR BTRIM(npsn) IN ('-', '11111111', '22222222', '33333333')
  );

CREATE UNIQUE INDEX uq_school_npsn_normalized
    ON school (UPPER(BTRIM(npsn)))
    WHERE npsn IS NOT NULL;
