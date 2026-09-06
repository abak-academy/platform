UPDATE school
SET npsn = NULL
WHERE npsn IS NOT NULL AND npsn !~ '[^[:space:]]';

CREATE UNIQUE INDEX uq_school_npsn_normalized
    ON school (UPPER(BTRIM(npsn)))
    WHERE npsn IS NOT NULL;
