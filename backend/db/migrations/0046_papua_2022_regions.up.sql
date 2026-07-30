-- 0046_papua_2022_regions.up.sql
-- Additive migration for FB-34 / H1: adds the four provinces created by the 2022/2023
-- Papua split (Papua Selatan, Papua Tengah, Papua Pegunungan, Papua Barat Daya) plus their
-- kabupaten/kota and kecamatan, per the current cahyadsn/wilayah mirror (Kepmendagri
-- 300.2.2-2138/2025 successor decree). Source data and full verification trail: task_15_finding.md.
--
-- Invariant 5: every existing province/city/district row is left exactly as-is --
-- orders.province_id/city_id/district_id (0033_shipping_ongkir.up.sql) FK into the existing
-- seed rows and must keep resolving to exactly what they resolved to before this migration.
--
-- Eleven official 2022+ codes collide with a *different* place already occupying that id in
-- the seed (1 province + city codes 9401/9402/9403/9404/9408 under it, plus six more city codes
-- under the still-untouched province 91 block that are out of this migration's insert scope).
-- Per FR36a, colliding entries take a surrogate id of the form LOC-<official_code> instead of
-- their true Kemendagri code. LOC- ids are local to this database only -- they are NOT official
-- Kemendagri wilayah codes and must never be presented as such. Non-colliding entries use their
-- true official code directly. See FR36a and task_15_finding.md's collision table for the full
-- rationale (province/city ids are unconstrained TEXT PKs; hierarchy is enforced by FK columns,
-- not by id format; region ids never reach Biteship, which is quoted by postal code).

-- 4 new provinces (FR36). Existing '91' (PAPUA BARAT) and '94' (PAPUA) are untouched.
INSERT INTO province (id, name) VALUES ('93', 'PAPUA SELATAN');
INSERT INTO province (id, name) VALUES ('LOC-94', 'PAPUA TENGAH'); -- surrogate: official code 94 already used by seed's PAPUA
INSERT INTO province (id, name) VALUES ('95', 'PAPUA PEGUNUNGAN');
INSERT INTO province (id, name) VALUES ('96', 'PAPUA BARAT DAYA');

-- 26 kabupaten/kota across the four new provinces.
-- province 93
INSERT INTO city (id, province_id, name) VALUES ('9301', '93', 'KABUPATEN MERAUKE');
INSERT INTO city (id, province_id, name) VALUES ('9302', '93', 'KABUPATEN BOVEN DIGOEL');
INSERT INTO city (id, province_id, name) VALUES ('9303', '93', 'KABUPATEN MAPPI');
INSERT INTO city (id, province_id, name) VALUES ('9304', '93', 'KABUPATEN ASMAT');

-- province LOC-94
INSERT INTO city (id, province_id, name) VALUES ('LOC-9401', 'LOC-94', 'KABUPATEN NABIRE');  -- surrogate: official code collides with existing seed city
INSERT INTO city (id, province_id, name) VALUES ('LOC-9402', 'LOC-94', 'KABUPATEN PUNCAK JAYA');  -- surrogate: official code collides with existing seed city
INSERT INTO city (id, province_id, name) VALUES ('LOC-9403', 'LOC-94', 'KABUPATEN PANIAI');  -- surrogate: official code collides with existing seed city
INSERT INTO city (id, province_id, name) VALUES ('LOC-9404', 'LOC-94', 'KABUPATEN MIMIKA');  -- surrogate: official code collides with existing seed city
INSERT INTO city (id, province_id, name) VALUES ('9405', 'LOC-94', 'KABUPATEN PUNCAK');
INSERT INTO city (id, province_id, name) VALUES ('9406', 'LOC-94', 'KABUPATEN DOGIYAI');
INSERT INTO city (id, province_id, name) VALUES ('9407', 'LOC-94', 'KABUPATEN INTAN JAYA');
INSERT INTO city (id, province_id, name) VALUES ('LOC-9408', 'LOC-94', 'KABUPATEN DEIYAI');  -- surrogate: official code collides with existing seed city

-- province 95
INSERT INTO city (id, province_id, name) VALUES ('9501', '95', 'KABUPATEN JAYAWIJAYA');
INSERT INTO city (id, province_id, name) VALUES ('9502', '95', 'KABUPATEN PEGUNUNGAN BINTANG');
INSERT INTO city (id, province_id, name) VALUES ('9503', '95', 'KABUPATEN YAHUKIMO');
INSERT INTO city (id, province_id, name) VALUES ('9504', '95', 'KABUPATEN TOLIKARA');
INSERT INTO city (id, province_id, name) VALUES ('9505', '95', 'KABUPATEN MAMBERAMO TENGAH');
INSERT INTO city (id, province_id, name) VALUES ('9506', '95', 'KABUPATEN YALIMO');
INSERT INTO city (id, province_id, name) VALUES ('9507', '95', 'KABUPATEN LANNY JAYA');
INSERT INTO city (id, province_id, name) VALUES ('9508', '95', 'KABUPATEN NDUGA');

-- province 96
INSERT INTO city (id, province_id, name) VALUES ('9601', '96', 'KABUPATEN SORONG');
INSERT INTO city (id, province_id, name) VALUES ('9602', '96', 'KABUPATEN SORONG SELATAN');
INSERT INTO city (id, province_id, name) VALUES ('9603', '96', 'KABUPATEN RAJA AMPAT');
INSERT INTO city (id, province_id, name) VALUES ('9604', '96', 'KABUPATEN TAMBRAUW');
INSERT INTO city (id, province_id, name) VALUES ('9605', '96', 'KABUPATEN MAYBRAT');
INSERT INTO city (id, province_id, name) VALUES ('9671', '96', 'KOTA SORONG');

-- 597 kecamatan across the 26 new kabupaten/kota.
-- 9301 KABUPATEN MERAUKE -- city 9301 (22 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('930101', '9301', 'MERAUKE');
INSERT INTO district (id, city_id, name) VALUES ('930102', '9301', 'MUTING');
INSERT INTO district (id, city_id, name) VALUES ('930103', '9301', 'OKABA');
INSERT INTO district (id, city_id, name) VALUES ('930104', '9301', 'KIMAAM');
INSERT INTO district (id, city_id, name) VALUES ('930105', '9301', 'SEMANGGA');
INSERT INTO district (id, city_id, name) VALUES ('930106', '9301', 'TANAH MIRING');
INSERT INTO district (id, city_id, name) VALUES ('930107', '9301', 'JAGEBOB');
INSERT INTO district (id, city_id, name) VALUES ('930108', '9301', 'SOTA');
INSERT INTO district (id, city_id, name) VALUES ('930109', '9301', 'ULILIN');
INSERT INTO district (id, city_id, name) VALUES ('930110', '9301', 'ELIKOBAL');
INSERT INTO district (id, city_id, name) VALUES ('930111', '9301', 'KURIK');
INSERT INTO district (id, city_id, name) VALUES ('930112', '9301', 'NAUKENJERAI');
INSERT INTO district (id, city_id, name) VALUES ('930113', '9301', 'ANIMHA');
INSERT INTO district (id, city_id, name) VALUES ('930114', '9301', 'MALIND');
INSERT INTO district (id, city_id, name) VALUES ('930115', '9301', 'TUBANG');
INSERT INTO district (id, city_id, name) VALUES ('930116', '9301', 'NGGUTI');
INSERT INTO district (id, city_id, name) VALUES ('930117', '9301', 'KAPTEL');
INSERT INTO district (id, city_id, name) VALUES ('930118', '9301', 'TABONJI');
INSERT INTO district (id, city_id, name) VALUES ('930119', '9301', 'WAAN');
INSERT INTO district (id, city_id, name) VALUES ('930120', '9301', 'ILWAYAB');
INSERT INTO district (id, city_id, name) VALUES ('930121', '9301', 'PADUA');
INSERT INTO district (id, city_id, name) VALUES ('930122', '9301', 'KONTUAR');

-- 9302 KABUPATEN BOVEN DIGOEL -- city 9302 (20 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('930201', '9302', 'MANDOBO');
INSERT INTO district (id, city_id, name) VALUES ('930202', '9302', 'MINDIPTANA');
INSERT INTO district (id, city_id, name) VALUES ('930203', '9302', 'WAROPKO');
INSERT INTO district (id, city_id, name) VALUES ('930204', '9302', 'KOUH');
INSERT INTO district (id, city_id, name) VALUES ('930205', '9302', 'JAIR');
INSERT INTO district (id, city_id, name) VALUES ('930206', '9302', 'BOMAKIA');
INSERT INTO district (id, city_id, name) VALUES ('930207', '9302', 'KOMBUT');
INSERT INTO district (id, city_id, name) VALUES ('930208', '9302', 'INIYANDIT');
INSERT INTO district (id, city_id, name) VALUES ('930209', '9302', 'ARIMOP');
INSERT INTO district (id, city_id, name) VALUES ('930210', '9302', 'FOFI');
INSERT INTO district (id, city_id, name) VALUES ('930211', '9302', 'AMBATKWI');
INSERT INTO district (id, city_id, name) VALUES ('930212', '9302', 'MANGGELUM');
INSERT INTO district (id, city_id, name) VALUES ('930213', '9302', 'FIRIWAGE');
INSERT INTO district (id, city_id, name) VALUES ('930214', '9302', 'YANIRUMA');
INSERT INTO district (id, city_id, name) VALUES ('930215', '9302', 'SUBUR');
INSERT INTO district (id, city_id, name) VALUES ('930216', '9302', 'KOMBAY');
INSERT INTO district (id, city_id, name) VALUES ('930217', '9302', 'NINATI');
INSERT INTO district (id, city_id, name) VALUES ('930218', '9302', 'SESNUK');
INSERT INTO district (id, city_id, name) VALUES ('930219', '9302', 'KI');
INSERT INTO district (id, city_id, name) VALUES ('930220', '9302', 'KAWAGIT');

-- 9303 KABUPATEN MAPPI -- city 9303 (15 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('930301', '9303', 'OBAA');
INSERT INTO district (id, city_id, name) VALUES ('930302', '9303', 'MAMBIOMAN BAPAI');
INSERT INTO district (id, city_id, name) VALUES ('930303', '9303', 'CITAK-MITAK');
INSERT INTO district (id, city_id, name) VALUES ('930304', '9303', 'EDERA');
INSERT INTO district (id, city_id, name) VALUES ('930305', '9303', 'HAJU');
INSERT INTO district (id, city_id, name) VALUES ('930306', '9303', 'ASSUE');
INSERT INTO district (id, city_id, name) VALUES ('930307', '9303', 'KAIBAR');
INSERT INTO district (id, city_id, name) VALUES ('930308', '9303', 'PASSUE');
INSERT INTO district (id, city_id, name) VALUES ('930309', '9303', 'MINYAMUR');
INSERT INTO district (id, city_id, name) VALUES ('930310', '9303', 'VENAHA');
INSERT INTO district (id, city_id, name) VALUES ('930311', '9303', 'SYAHCAME');
INSERT INTO district (id, city_id, name) VALUES ('930312', '9303', 'YAKOMI');
INSERT INTO district (id, city_id, name) VALUES ('930313', '9303', 'BAMGI');
INSERT INTO district (id, city_id, name) VALUES ('930314', '9303', 'PASSUE BAWAH');
INSERT INTO district (id, city_id, name) VALUES ('930315', '9303', 'TI ZAIN');

-- 9304 KABUPATEN ASMAT -- city 9304 (25 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('930401', '9304', 'AGATS');
INSERT INTO district (id, city_id, name) VALUES ('930402', '9304', 'ATSJ');
INSERT INTO district (id, city_id, name) VALUES ('930403', '9304', 'SAWA ERMA');
INSERT INTO district (id, city_id, name) VALUES ('930404', '9304', 'AKAT');
INSERT INTO district (id, city_id, name) VALUES ('930405', '9304', 'FAYIT');
INSERT INTO district (id, city_id, name) VALUES ('930406', '9304', 'PANTAI KASUARI');
INSERT INTO district (id, city_id, name) VALUES ('930407', '9304', 'SUATOR');
INSERT INTO district (id, city_id, name) VALUES ('930408', '9304', 'SURU-SURU');
INSERT INTO district (id, city_id, name) VALUES ('930409', '9304', 'KOLF BRAZA');
INSERT INTO district (id, city_id, name) VALUES ('930410', '9304', 'UNIR SIRAU');
INSERT INTO district (id, city_id, name) VALUES ('930411', '9304', 'JOERAT');
INSERT INTO district (id, city_id, name) VALUES ('930412', '9304', 'PULAU TIGA');
INSERT INTO district (id, city_id, name) VALUES ('930413', '9304', 'JETSY');
INSERT INTO district (id, city_id, name) VALUES ('930414', '9304', 'DER KOUMUR');
INSERT INTO district (id, city_id, name) VALUES ('930415', '9304', 'KOPAY');
INSERT INTO district (id, city_id, name) VALUES ('930416', '9304', 'SAFAN');
INSERT INTO district (id, city_id, name) VALUES ('930417', '9304', 'SIRETS');
INSERT INTO district (id, city_id, name) VALUES ('930418', '9304', 'AYIB');
INSERT INTO district (id, city_id, name) VALUES ('930419', '9304', 'BETCBAMU');
INSERT INTO district (id, city_id, name) VALUES ('930420', '9304', 'JOUTU');
INSERT INTO district (id, city_id, name) VALUES ('930421', '9304', 'ASWI');
INSERT INTO district (id, city_id, name) VALUES ('930422', '9304', 'AWYU');
INSERT INTO district (id, city_id, name) VALUES ('930423', '9304', 'KOROWAY BULUANOP');
INSERT INTO district (id, city_id, name) VALUES ('930424', '9304', 'TOMOR BIRIP');
INSERT INTO district (id, city_id, name) VALUES ('930425', '9304', 'SOR EP');

-- 9401 KABUPATEN NABIRE -- city LOC-9401 (15 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940101', 'LOC-9401', 'NABIRE');
INSERT INTO district (id, city_id, name) VALUES ('940102', 'LOC-9401', 'NAPAN');
INSERT INTO district (id, city_id, name) VALUES ('940103', 'LOC-9401', 'YAUR');
INSERT INTO district (id, city_id, name) VALUES ('940104', 'LOC-9401', 'UWAPA');
INSERT INTO district (id, city_id, name) VALUES ('940105', 'LOC-9401', 'WANGGAR');
INSERT INTO district (id, city_id, name) VALUES ('940106', 'LOC-9401', 'SIRIWO');
INSERT INTO district (id, city_id, name) VALUES ('940107', 'LOC-9401', 'MAKIMI');
INSERT INTO district (id, city_id, name) VALUES ('940108', 'LOC-9401', 'TELUK UMAR');
INSERT INTO district (id, city_id, name) VALUES ('940109', 'LOC-9401', 'TELUK KIMI');
INSERT INTO district (id, city_id, name) VALUES ('940110', 'LOC-9401', 'YARO');
INSERT INTO district (id, city_id, name) VALUES ('940111', 'LOC-9401', 'WAPOGA');
INSERT INTO district (id, city_id, name) VALUES ('940112', 'LOC-9401', 'NABIRE BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940113', 'LOC-9401', 'MOORA');
INSERT INTO district (id, city_id, name) VALUES ('940114', 'LOC-9401', 'DIPA');
INSERT INTO district (id, city_id, name) VALUES ('940115', 'LOC-9401', 'MENOU');

-- 9402 KABUPATEN PUNCAK JAYA -- city LOC-9402 (26 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940201', 'LOC-9402', 'MULIA');
INSERT INTO district (id, city_id, name) VALUES ('940202', 'LOC-9402', 'ILU');
INSERT INTO district (id, city_id, name) VALUES ('940203', 'LOC-9402', 'FAWI');
INSERT INTO district (id, city_id, name) VALUES ('940204', 'LOC-9402', 'MEWOLUK');
INSERT INTO district (id, city_id, name) VALUES ('940205', 'LOC-9402', 'YAMO');
INSERT INTO district (id, city_id, name) VALUES ('940206', 'LOC-9402', 'NUME');
INSERT INTO district (id, city_id, name) VALUES ('940207', 'LOC-9402', 'TORERE');
INSERT INTO district (id, city_id, name) VALUES ('940208', 'LOC-9402', 'TINGGINAMBUT');
INSERT INTO district (id, city_id, name) VALUES ('940209', 'LOC-9402', 'PAGALEME');
INSERT INTO district (id, city_id, name) VALUES ('940210', 'LOC-9402', 'GURAGE');
INSERT INTO district (id, city_id, name) VALUES ('940211', 'LOC-9402', 'IRIMULI');
INSERT INTO district (id, city_id, name) VALUES ('940212', 'LOC-9402', 'MUARA');
INSERT INTO district (id, city_id, name) VALUES ('940213', 'LOC-9402', 'ILAMBURAWI');
INSERT INTO district (id, city_id, name) VALUES ('940214', 'LOC-9402', 'YAMBI');
INSERT INTO district (id, city_id, name) VALUES ('940215', 'LOC-9402', 'LUMO');
INSERT INTO district (id, city_id, name) VALUES ('940216', 'LOC-9402', 'MOLANIKIME');
INSERT INTO district (id, city_id, name) VALUES ('940217', 'LOC-9402', 'DOKOME');
INSERT INTO district (id, city_id, name) VALUES ('940218', 'LOC-9402', 'KALOME');
INSERT INTO district (id, city_id, name) VALUES ('940219', 'LOC-9402', 'WANWI');
INSERT INTO district (id, city_id, name) VALUES ('940220', 'LOC-9402', 'YAMONERI');
INSERT INTO district (id, city_id, name) VALUES ('940221', 'LOC-9402', 'WAEGI');
INSERT INTO district (id, city_id, name) VALUES ('940222', 'LOC-9402', 'NIOGA');
INSERT INTO district (id, city_id, name) VALUES ('940223', 'LOC-9402', 'GUBUME');
INSERT INTO district (id, city_id, name) VALUES ('940224', 'LOC-9402', 'TAGANOMBAK');
INSERT INTO district (id, city_id, name) VALUES ('940225', 'LOC-9402', 'DAGAI');
INSERT INTO district (id, city_id, name) VALUES ('940226', 'LOC-9402', 'KIYAGE');

-- 9403 KABUPATEN PANIAI -- city LOC-9403 (24 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940301', 'LOC-9403', 'PANIAI TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('940302', 'LOC-9403', 'PANIAI BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940303', 'LOC-9403', 'ARADIDE');
INSERT INTO district (id, city_id, name) VALUES ('940304', 'LOC-9403', 'BOGABAIDA');
INSERT INTO district (id, city_id, name) VALUES ('940305', 'LOC-9403', 'BIBIDA');
INSERT INTO district (id, city_id, name) VALUES ('940306', 'LOC-9403', 'DUMADAMA');
INSERT INTO district (id, city_id, name) VALUES ('940307', 'LOC-9403', 'SIRIWO');
INSERT INTO district (id, city_id, name) VALUES ('940308', 'LOC-9403', 'KEBO');
INSERT INTO district (id, city_id, name) VALUES ('940309', 'LOC-9403', 'YATAMO');
INSERT INTO district (id, city_id, name) VALUES ('940310', 'LOC-9403', 'EKADIDE');
INSERT INTO district (id, city_id, name) VALUES ('940311', 'LOC-9403', 'WEGEE MUKA');
INSERT INTO district (id, city_id, name) VALUES ('940312', 'LOC-9403', 'WEGEE BINO');
INSERT INTO district (id, city_id, name) VALUES ('940313', 'LOC-9403', 'PUGO DAGI');
INSERT INTO district (id, city_id, name) VALUES ('940314', 'LOC-9403', 'MUYE');
INSERT INTO district (id, city_id, name) VALUES ('940315', 'LOC-9403', 'NAKAMA');
INSERT INTO district (id, city_id, name) VALUES ('940316', 'LOC-9403', 'TELUK DEYA');
INSERT INTO district (id, city_id, name) VALUES ('940317', 'LOC-9403', 'YAGAI');
INSERT INTO district (id, city_id, name) VALUES ('940318', 'LOC-9403', 'YOUTADI');
INSERT INTO district (id, city_id, name) VALUES ('940319', 'LOC-9403', 'BAYA BIRU');
INSERT INTO district (id, city_id, name) VALUES ('940320', 'LOC-9403', 'DEIYAI MIYO');
INSERT INTO district (id, city_id, name) VALUES ('940321', 'LOC-9403', 'DOGOMO');
INSERT INTO district (id, city_id, name) VALUES ('940322', 'LOC-9403', 'AWEIDA');
INSERT INTO district (id, city_id, name) VALUES ('940323', 'LOC-9403', 'TOPIYAI');
INSERT INTO district (id, city_id, name) VALUES ('940324', 'LOC-9403', 'FAJAR TIMUR');

-- 9404 KABUPATEN MIMIKA -- city LOC-9404 (18 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940401', 'LOC-9404', 'MIMIKA BARU');
INSERT INTO district (id, city_id, name) VALUES ('940402', 'LOC-9404', 'AGIMUGA');
INSERT INTO district (id, city_id, name) VALUES ('940403', 'LOC-9404', 'MIMIKA TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('940404', 'LOC-9404', 'MIMIKA BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940405', 'LOC-9404', 'JITA');
INSERT INTO district (id, city_id, name) VALUES ('940406', 'LOC-9404', 'JILA');
INSERT INTO district (id, city_id, name) VALUES ('940407', 'LOC-9404', 'MIMIKA TIMUR JAUH');
INSERT INTO district (id, city_id, name) VALUES ('940408', 'LOC-9404', 'MIMIKA TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('940409', 'LOC-9404', 'KUALA KENCANA');
INSERT INTO district (id, city_id, name) VALUES ('940410', 'LOC-9404', 'TEMBAGAPURA');
INSERT INTO district (id, city_id, name) VALUES ('940411', 'LOC-9404', 'MIMIKA BARAT JAUH');
INSERT INTO district (id, city_id, name) VALUES ('940412', 'LOC-9404', 'MIMIKA BARAT TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('940413', 'LOC-9404', 'KWAMKI NARAMA');
INSERT INTO district (id, city_id, name) VALUES ('940414', 'LOC-9404', 'HOYA');
INSERT INTO district (id, city_id, name) VALUES ('940415', 'LOC-9404', 'IWAKA');
INSERT INTO district (id, city_id, name) VALUES ('940416', 'LOC-9404', 'WANIA');
INSERT INTO district (id, city_id, name) VALUES ('940417', 'LOC-9404', 'AMAR');
INSERT INTO district (id, city_id, name) VALUES ('940418', 'LOC-9404', 'ALAMA');

-- 9405 KABUPATEN PUNCAK -- city 9405 (25 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940501', '9405', 'ILAGA');
INSERT INTO district (id, city_id, name) VALUES ('940502', '9405', 'WANGBE');
INSERT INTO district (id, city_id, name) VALUES ('940503', '9405', 'BEOGA');
INSERT INTO district (id, city_id, name) VALUES ('940504', '9405', 'DOUFO');
INSERT INTO district (id, city_id, name) VALUES ('940505', '9405', 'POGOMA');
INSERT INTO district (id, city_id, name) VALUES ('940506', '9405', 'SINAK');
INSERT INTO district (id, city_id, name) VALUES ('940507', '9405', 'AGANDUGUME');
INSERT INTO district (id, city_id, name) VALUES ('940508', '9405', 'GOME');
INSERT INTO district (id, city_id, name) VALUES ('940509', '9405', 'DERVOS');
INSERT INTO district (id, city_id, name) VALUES ('940510', '9405', 'BEOGA BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940511', '9405', 'BEOGA TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('940512', '9405', 'OGAMANIM');
INSERT INTO district (id, city_id, name) VALUES ('940513', '9405', 'KEMBRU');
INSERT INTO district (id, city_id, name) VALUES ('940514', '9405', 'BINA');
INSERT INTO district (id, city_id, name) VALUES ('940515', '9405', 'SINAK BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940516', '9405', 'MAGE''ABUME');
INSERT INTO district (id, city_id, name) VALUES ('940517', '9405', 'YUGUMUAK');
INSERT INTO district (id, city_id, name) VALUES ('940518', '9405', 'ILAGA UTARA');
INSERT INTO district (id, city_id, name) VALUES ('940519', '9405', 'MABUGI');
INSERT INTO district (id, city_id, name) VALUES ('940520', '9405', 'OMUKIA');
INSERT INTO district (id, city_id, name) VALUES ('940521', '9405', 'LAMBEWI');
INSERT INTO district (id, city_id, name) VALUES ('940522', '9405', 'ONERI');
INSERT INTO district (id, city_id, name) VALUES ('940523', '9405', 'AMUNGKALPIA');
INSERT INTO district (id, city_id, name) VALUES ('940524', '9405', 'GOME UTARA');
INSERT INTO district (id, city_id, name) VALUES ('940525', '9405', 'ERELMAKAWIA');

-- 9406 KABUPATEN DOGIYAI -- city 9406 (10 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940601', '9406', 'KAMU');
INSERT INTO district (id, city_id, name) VALUES ('940602', '9406', 'MAPIA');
INSERT INTO district (id, city_id, name) VALUES ('940603', '9406', 'PIYAIYE');
INSERT INTO district (id, city_id, name) VALUES ('940604', '9406', 'KAMU UTARA');
INSERT INTO district (id, city_id, name) VALUES ('940605', '9406', 'SUKIKAI SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('940606', '9406', 'MAPIA BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940607', '9406', 'KAMU SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('940608', '9406', 'KAMU TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('940609', '9406', 'MAPIA TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('940610', '9406', 'DOGIYAI');

-- 9407 KABUPATEN INTAN JAYA -- city 9407 (8 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940701', '9407', 'SUGAPA');
INSERT INTO district (id, city_id, name) VALUES ('940702', '9407', 'HOMEYO');
INSERT INTO district (id, city_id, name) VALUES ('940703', '9407', 'WANDAI');
INSERT INTO district (id, city_id, name) VALUES ('940704', '9407', 'BIANDOGA');
INSERT INTO district (id, city_id, name) VALUES ('940705', '9407', 'AGISIGA');
INSERT INTO district (id, city_id, name) VALUES ('940706', '9407', 'HITADIPA');
INSERT INTO district (id, city_id, name) VALUES ('940707', '9407', 'UGIMBA');
INSERT INTO district (id, city_id, name) VALUES ('940708', '9407', 'TOMOSIGA');

-- 9408 KABUPATEN DEIYAI -- city LOC-9408 (5 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('940801', 'LOC-9408', 'TIGI');
INSERT INTO district (id, city_id, name) VALUES ('940802', 'LOC-9408', 'TIGI TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('940803', 'LOC-9408', 'BOWOBADO');
INSERT INTO district (id, city_id, name) VALUES ('940804', 'LOC-9408', 'TIGI BARAT');
INSERT INTO district (id, city_id, name) VALUES ('940805', 'LOC-9408', 'KAPIRAYA');

-- 9501 KABUPATEN JAYAWIJAYA -- city 9501 (40 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950101', '9501', 'WAMENA');
INSERT INTO district (id, city_id, name) VALUES ('950102', '9501', 'KURULU');
INSERT INTO district (id, city_id, name) VALUES ('950103', '9501', 'ASOLOGAIMA');
INSERT INTO district (id, city_id, name) VALUES ('950104', '9501', 'HUBIKOSI');
INSERT INTO district (id, city_id, name) VALUES ('950105', '9501', 'BOLAKME');
INSERT INTO district (id, city_id, name) VALUES ('950106', '9501', 'WALELAGAMA');
INSERT INTO district (id, city_id, name) VALUES ('950107', '9501', 'MUSATFAK');
INSERT INTO district (id, city_id, name) VALUES ('950108', '9501', 'WOLO');
INSERT INTO district (id, city_id, name) VALUES ('950109', '9501', 'ASOLOKOBAL');
INSERT INTO district (id, city_id, name) VALUES ('950110', '9501', 'PELEBAGA');
INSERT INTO district (id, city_id, name) VALUES ('950111', '9501', 'YALENGGA');
INSERT INTO district (id, city_id, name) VALUES ('950112', '9501', 'TRIKORA');
INSERT INTO district (id, city_id, name) VALUES ('950113', '9501', 'NAPUA');
INSERT INTO district (id, city_id, name) VALUES ('950114', '9501', 'WALAIK');
INSERT INTO district (id, city_id, name) VALUES ('950115', '9501', 'WOUMA');
INSERT INTO district (id, city_id, name) VALUES ('950116', '9501', 'HUBIKIAK');
INSERT INTO district (id, city_id, name) VALUES ('950117', '9501', 'IBELE');
INSERT INTO district (id, city_id, name) VALUES ('950118', '9501', 'TAELAREK');
INSERT INTO district (id, city_id, name) VALUES ('950119', '9501', 'ITLAY HISAGE');
INSERT INTO district (id, city_id, name) VALUES ('950120', '9501', 'SIEPKOSI');
INSERT INTO district (id, city_id, name) VALUES ('950121', '9501', 'USILIMO');
INSERT INTO district (id, city_id, name) VALUES ('950122', '9501', 'WITA WAYA');
INSERT INTO district (id, city_id, name) VALUES ('950123', '9501', 'LIBAREK');
INSERT INTO district (id, city_id, name) VALUES ('950124', '9501', 'WADANGKU');
INSERT INTO district (id, city_id, name) VALUES ('950125', '9501', 'PISUGI');
INSERT INTO district (id, city_id, name) VALUES ('950126', '9501', 'KORAGI');
INSERT INTO district (id, city_id, name) VALUES ('950127', '9501', 'TAGIME');
INSERT INTO district (id, city_id, name) VALUES ('950128', '9501', 'MOLAGALOME');
INSERT INTO district (id, city_id, name) VALUES ('950129', '9501', 'TAGINERI');
INSERT INTO district (id, city_id, name) VALUES ('950130', '9501', 'SILO KARNO DOGA');
INSERT INTO district (id, city_id, name) VALUES ('950131', '9501', 'PIRAMID');
INSERT INTO district (id, city_id, name) VALUES ('950132', '9501', 'MULIAMA');
INSERT INTO district (id, city_id, name) VALUES ('950133', '9501', 'BUGI');
INSERT INTO district (id, city_id, name) VALUES ('950134', '9501', 'BPIRI');
INSERT INTO district (id, city_id, name) VALUES ('950135', '9501', 'WELESI');
INSERT INTO district (id, city_id, name) VALUES ('950136', '9501', 'ASOTIPO');
INSERT INTO district (id, city_id, name) VALUES ('950137', '9501', 'MAIMA');
INSERT INTO district (id, city_id, name) VALUES ('950138', '9501', 'POPUGOBA');
INSERT INTO district (id, city_id, name) VALUES ('950139', '9501', 'WAME');
INSERT INTO district (id, city_id, name) VALUES ('950140', '9501', 'WESAPUT');

-- 9502 KABUPATEN PEGUNUNGAN BINTANG -- city 9502 (34 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950201', '9502', 'OKSIBIL');
INSERT INTO district (id, city_id, name) VALUES ('950202', '9502', 'KIWIROK');
INSERT INTO district (id, city_id, name) VALUES ('950203', '9502', 'OKBIBAB');
INSERT INTO district (id, city_id, name) VALUES ('950204', '9502', 'IWUR');
INSERT INTO district (id, city_id, name) VALUES ('950205', '9502', 'BATOM');
INSERT INTO district (id, city_id, name) VALUES ('950206', '9502', 'BORME');
INSERT INTO district (id, city_id, name) VALUES ('950207', '9502', 'KIWIROK TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('950208', '9502', 'ABOY');
INSERT INTO district (id, city_id, name) VALUES ('950209', '9502', 'PEPERA');
INSERT INTO district (id, city_id, name) VALUES ('950210', '9502', 'BIME');
INSERT INTO district (id, city_id, name) VALUES ('950211', '9502', 'ALEMSOM');
INSERT INTO district (id, city_id, name) VALUES ('950212', '9502', 'OKBAPE');
INSERT INTO district (id, city_id, name) VALUES ('950213', '9502', 'KALOMDOL');
INSERT INTO district (id, city_id, name) VALUES ('950214', '9502', 'OKSOP');
INSERT INTO district (id, city_id, name) VALUES ('950215', '9502', 'SERAMBAKON');
INSERT INTO district (id, city_id, name) VALUES ('950216', '9502', 'OK AOM');
INSERT INTO district (id, city_id, name) VALUES ('950217', '9502', 'KAWOR');
INSERT INTO district (id, city_id, name) VALUES ('950218', '9502', 'AWINBON');
INSERT INTO district (id, city_id, name) VALUES ('950219', '9502', 'TARUP');
INSERT INTO district (id, city_id, name) VALUES ('950220', '9502', 'OKHIKA');
INSERT INTO district (id, city_id, name) VALUES ('950221', '9502', 'OKSAMOL');
INSERT INTO district (id, city_id, name) VALUES ('950222', '9502', 'OKLIP');
INSERT INTO district (id, city_id, name) VALUES ('950223', '9502', 'OKBEMTAU');
INSERT INTO district (id, city_id, name) VALUES ('950224', '9502', 'OKSEBANG');
INSERT INTO district (id, city_id, name) VALUES ('950225', '9502', 'OKBAB');
INSERT INTO district (id, city_id, name) VALUES ('950226', '9502', 'BATANI');
INSERT INTO district (id, city_id, name) VALUES ('950227', '9502', 'WEIME');
INSERT INTO district (id, city_id, name) VALUES ('950228', '9502', 'MURKIM');
INSERT INTO district (id, city_id, name) VALUES ('950229', '9502', 'MOFINOP');
INSERT INTO district (id, city_id, name) VALUES ('950230', '9502', 'JETFA');
INSERT INTO district (id, city_id, name) VALUES ('950231', '9502', 'TEIRAPLU');
INSERT INTO district (id, city_id, name) VALUES ('950232', '9502', 'EIPUMEK');
INSERT INTO district (id, city_id, name) VALUES ('950233', '9502', 'PAMEK');
INSERT INTO district (id, city_id, name) VALUES ('950234', '9502', 'NONGME');

-- 9503 KABUPATEN YAHUKIMO -- city 9503 (51 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950301', '9503', 'KURIMA');
INSERT INTO district (id, city_id, name) VALUES ('950302', '9503', 'ANGGRUK');
INSERT INTO district (id, city_id, name) VALUES ('950303', '9503', 'NINIA');
INSERT INTO district (id, city_id, name) VALUES ('950304', '9503', 'SILIMO');
INSERT INTO district (id, city_id, name) VALUES ('950305', '9503', 'SAMENAGE');
INSERT INTO district (id, city_id, name) VALUES ('950306', '9503', 'NALCA');
INSERT INTO district (id, city_id, name) VALUES ('950307', '9503', 'DEKAI');
INSERT INTO district (id, city_id, name) VALUES ('950308', '9503', 'OBIO');
INSERT INTO district (id, city_id, name) VALUES ('950309', '9503', 'SURU SURU');
INSERT INTO district (id, city_id, name) VALUES ('950310', '9503', 'WUSAMA');
INSERT INTO district (id, city_id, name) VALUES ('950311', '9503', 'AMUMA');
INSERT INTO district (id, city_id, name) VALUES ('950312', '9503', 'MUSAIK');
INSERT INTO district (id, city_id, name) VALUES ('950313', '9503', 'PASEMA');
INSERT INTO district (id, city_id, name) VALUES ('950314', '9503', 'HOGIO');
INSERT INTO district (id, city_id, name) VALUES ('950315', '9503', 'MUGI');
INSERT INTO district (id, city_id, name) VALUES ('950316', '9503', 'SOBA');
INSERT INTO district (id, city_id, name) VALUES ('950317', '9503', 'WERIMA');
INSERT INTO district (id, city_id, name) VALUES ('950318', '9503', 'TANGMA');
INSERT INTO district (id, city_id, name) VALUES ('950319', '9503', 'UKHA');
INSERT INTO district (id, city_id, name) VALUES ('950320', '9503', 'PANGGEMA');
INSERT INTO district (id, city_id, name) VALUES ('950321', '9503', 'KOSAREK');
INSERT INTO district (id, city_id, name) VALUES ('950322', '9503', 'NIPSAN');
INSERT INTO district (id, city_id, name) VALUES ('950323', '9503', 'UBAHAK');
INSERT INTO district (id, city_id, name) VALUES ('950324', '9503', 'PRONGGOLI');
INSERT INTO district (id, city_id, name) VALUES ('950325', '9503', 'WALMA');
INSERT INTO district (id, city_id, name) VALUES ('950326', '9503', 'YAHULIAMBUT');
INSERT INTO district (id, city_id, name) VALUES ('950327', '9503', 'HEREAPINI');
INSERT INTO district (id, city_id, name) VALUES ('950328', '9503', 'UBALIHI');
INSERT INTO district (id, city_id, name) VALUES ('950329', '9503', 'TALAMBO');
INSERT INTO district (id, city_id, name) VALUES ('950330', '9503', 'PULDAMA');
INSERT INTO district (id, city_id, name) VALUES ('950331', '9503', 'ENDOMEN');
INSERT INTO district (id, city_id, name) VALUES ('950332', '9503', 'KONA');
INSERT INTO district (id, city_id, name) VALUES ('950333', '9503', 'DIRWEMNA');
INSERT INTO district (id, city_id, name) VALUES ('950334', '9503', 'HOLUWON');
INSERT INTO district (id, city_id, name) VALUES ('950335', '9503', 'LOLAT');
INSERT INTO district (id, city_id, name) VALUES ('950336', '9503', 'SOLOIKMA');
INSERT INTO district (id, city_id, name) VALUES ('950337', '9503', 'SELA');
INSERT INTO district (id, city_id, name) VALUES ('950338', '9503', 'KORUPUN');
INSERT INTO district (id, city_id, name) VALUES ('950339', '9503', 'LANGDA');
INSERT INTO district (id, city_id, name) VALUES ('950340', '9503', 'BOMELA');
INSERT INTO district (id, city_id, name) VALUES ('950341', '9503', 'SUNTAMON');
INSERT INTO district (id, city_id, name) VALUES ('950342', '9503', 'SERADALA');
INSERT INTO district (id, city_id, name) VALUES ('950343', '9503', 'SOBAHAM');
INSERT INTO district (id, city_id, name) VALUES ('950344', '9503', 'KABIANGGAMA');
INSERT INTO district (id, city_id, name) VALUES ('950345', '9503', 'KWELAMDUA');
INSERT INTO district (id, city_id, name) VALUES ('950346', '9503', 'KWIKMA');
INSERT INTO district (id, city_id, name) VALUES ('950347', '9503', 'HILIPUK');
INSERT INTO district (id, city_id, name) VALUES ('950348', '9503', 'DURAM');
INSERT INTO district (id, city_id, name) VALUES ('950349', '9503', 'YOGOSEM');
INSERT INTO district (id, city_id, name) VALUES ('950350', '9503', 'KAYO');
INSERT INTO district (id, city_id, name) VALUES ('950351', '9503', 'SUMO');

-- 9504 KABUPATEN TOLIKARA -- city 9504 (46 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950401', '9504', 'KARUBAGA');
INSERT INTO district (id, city_id, name) VALUES ('950402', '9504', 'BOKONDINI');
INSERT INTO district (id, city_id, name) VALUES ('950403', '9504', 'KANGGIME');
INSERT INTO district (id, city_id, name) VALUES ('950404', '9504', 'KEMBU');
INSERT INTO district (id, city_id, name) VALUES ('950405', '9504', 'GOYAGE');
INSERT INTO district (id, city_id, name) VALUES ('950406', '9504', 'WUNIM');
INSERT INTO district (id, city_id, name) VALUES ('950407', '9504', 'WINA');
INSERT INTO district (id, city_id, name) VALUES ('950408', '9504', 'UMAGI');
INSERT INTO district (id, city_id, name) VALUES ('950409', '9504', 'PANAGA');
INSERT INTO district (id, city_id, name) VALUES ('950410', '9504', 'WONIKI');
INSERT INTO district (id, city_id, name) VALUES ('950411', '9504', 'KUBU');
INSERT INTO district (id, city_id, name) VALUES ('950412', '9504', 'KONDA/KONDAGA');
INSERT INTO district (id, city_id, name) VALUES ('950413', '9504', 'NELAWI');
INSERT INTO district (id, city_id, name) VALUES ('950414', '9504', 'KUARI');
INSERT INTO district (id, city_id, name) VALUES ('950415', '9504', 'BOKONERI');
INSERT INTO district (id, city_id, name) VALUES ('950416', '9504', 'BEWANI');
INSERT INTO district (id, city_id, name) VALUES ('950417', '9504', 'NABUNAGE');
INSERT INTO district (id, city_id, name) VALUES ('950418', '9504', 'GILUBANDU');
INSERT INTO district (id, city_id, name) VALUES ('950419', '9504', 'NUNGGAWI');
INSERT INTO district (id, city_id, name) VALUES ('950420', '9504', 'GUNDAGI');
INSERT INTO district (id, city_id, name) VALUES ('950421', '9504', 'NUMBA');
INSERT INTO district (id, city_id, name) VALUES ('950422', '9504', 'TIMORI');
INSERT INTO district (id, city_id, name) VALUES ('950423', '9504', 'DUNDU');
INSERT INTO district (id, city_id, name) VALUES ('950424', '9504', 'GEYA');
INSERT INTO district (id, city_id, name) VALUES ('950425', '9504', 'EGIAM');
INSERT INTO district (id, city_id, name) VALUES ('950426', '9504', 'POGANERI');
INSERT INTO district (id, city_id, name) VALUES ('950427', '9504', 'KAMBONERI');
INSERT INTO district (id, city_id, name) VALUES ('950428', '9504', 'AIRGARAM');
INSERT INTO district (id, city_id, name) VALUES ('950429', '9504', 'WARI/TAIYEVE II');
INSERT INTO district (id, city_id, name) VALUES ('950430', '9504', 'DOW');
INSERT INTO district (id, city_id, name) VALUES ('950431', '9504', 'TAGINERI');
INSERT INTO district (id, city_id, name) VALUES ('950432', '9504', 'YUNERI');
INSERT INTO district (id, city_id, name) VALUES ('950433', '9504', 'WAKUWO');
INSERT INTO district (id, city_id, name) VALUES ('950434', '9504', 'GIKA');
INSERT INTO district (id, city_id, name) VALUES ('950435', '9504', 'TELENGGEME');
INSERT INTO district (id, city_id, name) VALUES ('950436', '9504', 'ANAWI');
INSERT INTO district (id, city_id, name) VALUES ('950437', '9504', 'WENAM');
INSERT INTO district (id, city_id, name) VALUES ('950438', '9504', 'WUGI');
INSERT INTO district (id, city_id, name) VALUES ('950439', '9504', 'DANIME');
INSERT INTO district (id, city_id, name) VALUES ('950440', '9504', 'TAGIME');
INSERT INTO district (id, city_id, name) VALUES ('950441', '9504', 'KAI');
INSERT INTO district (id, city_id, name) VALUES ('950442', '9504', 'AWEKU');
INSERT INTO district (id, city_id, name) VALUES ('950443', '9504', 'BOGONUK');
INSERT INTO district (id, city_id, name) VALUES ('950444', '9504', 'LI ANOGOMMA');
INSERT INTO district (id, city_id, name) VALUES ('950445', '9504', 'BIUK');
INSERT INTO district (id, city_id, name) VALUES ('950446', '9504', 'YUKO');

-- 9505 KABUPATEN MAMBERAMO TENGAH -- city 9505 (5 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950501', '9505', 'KOBAKMA');
INSERT INTO district (id, city_id, name) VALUES ('950502', '9505', 'KELILA');
INSERT INTO district (id, city_id, name) VALUES ('950503', '9505', 'ERAGAYAM');
INSERT INTO district (id, city_id, name) VALUES ('950504', '9505', 'MEGAMBILIS');
INSERT INTO district (id, city_id, name) VALUES ('950505', '9505', 'ILUGWA');

-- 9506 KABUPATEN YALIMO -- city 9506 (5 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950601', '9506', 'ELELIM');
INSERT INTO district (id, city_id, name) VALUES ('950602', '9506', 'APALAPSILI');
INSERT INTO district (id, city_id, name) VALUES ('950603', '9506', 'ABENAHO');
INSERT INTO district (id, city_id, name) VALUES ('950604', '9506', 'BENAWA');
INSERT INTO district (id, city_id, name) VALUES ('950605', '9506', 'WELAREK');

-- 9507 KABUPATEN LANNY JAYA -- city 9507 (39 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950701', '9507', 'TIOM');
INSERT INTO district (id, city_id, name) VALUES ('950702', '9507', 'PIRIME');
INSERT INTO district (id, city_id, name) VALUES ('950703', '9507', 'MAKKI');
INSERT INTO district (id, city_id, name) VALUES ('950704', '9507', 'GAMELIA');
INSERT INTO district (id, city_id, name) VALUES ('950705', '9507', 'DIMBA');
INSERT INTO district (id, city_id, name) VALUES ('950706', '9507', 'MELAGINERI');
INSERT INTO district (id, city_id, name) VALUES ('950707', '9507', 'BALINGGA');
INSERT INTO district (id, city_id, name) VALUES ('950708', '9507', 'TIOMNERI');
INSERT INTO district (id, city_id, name) VALUES ('950709', '9507', 'KUYAWAGE');
INSERT INTO district (id, city_id, name) VALUES ('950710', '9507', 'POGA');
INSERT INTO district (id, city_id, name) VALUES ('950711', '9507', 'NINAME');
INSERT INTO district (id, city_id, name) VALUES ('950712', '9507', 'NOGI');
INSERT INTO district (id, city_id, name) VALUES ('950713', '9507', 'YIGINUA');
INSERT INTO district (id, city_id, name) VALUES ('950714', '9507', 'TIOM OLLO');
INSERT INTO district (id, city_id, name) VALUES ('950715', '9507', 'YUGUNGWI');
INSERT INTO district (id, city_id, name) VALUES ('950716', '9507', 'MOKONI');
INSERT INTO district (id, city_id, name) VALUES ('950717', '9507', 'WEREKA');
INSERT INTO district (id, city_id, name) VALUES ('950718', '9507', 'MILIMBO');
INSERT INTO district (id, city_id, name) VALUES ('950719', '9507', 'WIRINGGAMBUT');
INSERT INTO district (id, city_id, name) VALUES ('950720', '9507', 'GOLLO');
INSERT INTO district (id, city_id, name) VALUES ('950721', '9507', 'AWINA');
INSERT INTO district (id, city_id, name) VALUES ('950722', '9507', 'AYUMNATI');
INSERT INTO district (id, city_id, name) VALUES ('950723', '9507', 'WANO BARAT');
INSERT INTO district (id, city_id, name) VALUES ('950724', '9507', 'GOA BALIM');
INSERT INTO district (id, city_id, name) VALUES ('950725', '9507', 'BRUWA');
INSERT INTO district (id, city_id, name) VALUES ('950726', '9507', 'BALINGGA BARAT');
INSERT INTO district (id, city_id, name) VALUES ('950727', '9507', 'GUPURA');
INSERT INTO district (id, city_id, name) VALUES ('950728', '9507', 'KOLAWA');
INSERT INTO district (id, city_id, name) VALUES ('950729', '9507', 'GELOK BEAM');
INSERT INTO district (id, city_id, name) VALUES ('950730', '9507', 'KULY LANNY');
INSERT INTO district (id, city_id, name) VALUES ('950731', '9507', 'LANNYNA');
INSERT INTO district (id, city_id, name) VALUES ('950732', '9507', 'KARU');
INSERT INTO district (id, city_id, name) VALUES ('950733', '9507', 'YILUK');
INSERT INTO district (id, city_id, name) VALUES ('950734', '9507', 'GUNA');
INSERT INTO district (id, city_id, name) VALUES ('950735', '9507', 'KELULOME');
INSERT INTO district (id, city_id, name) VALUES ('950736', '9507', 'NIKOGWE');
INSERT INTO district (id, city_id, name) VALUES ('950737', '9507', 'MUARA');
INSERT INTO district (id, city_id, name) VALUES ('950738', '9507', 'BUGUK GONA');
INSERT INTO district (id, city_id, name) VALUES ('950739', '9507', 'MELAGI');

-- 9508 KABUPATEN NDUGA -- city 9508 (32 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('950801', '9508', 'KENYAM');
INSERT INTO district (id, city_id, name) VALUES ('950802', '9508', 'MAPENDUMA');
INSERT INTO district (id, city_id, name) VALUES ('950803', '9508', 'YIGI');
INSERT INTO district (id, city_id, name) VALUES ('950804', '9508', 'WOSAK');
INSERT INTO district (id, city_id, name) VALUES ('950805', '9508', 'GESELMA');
INSERT INTO district (id, city_id, name) VALUES ('950806', '9508', 'MUGI');
INSERT INTO district (id, city_id, name) VALUES ('950807', '9508', 'MBUWA');
INSERT INTO district (id, city_id, name) VALUES ('950808', '9508', 'GEAREK');
INSERT INTO district (id, city_id, name) VALUES ('950809', '9508', 'KOROPTAK');
INSERT INTO district (id, city_id, name) VALUES ('950810', '9508', 'KEGAYEM');
INSERT INTO district (id, city_id, name) VALUES ('950811', '9508', 'PARO');
INSERT INTO district (id, city_id, name) VALUES ('950812', '9508', 'MEBAROK');
INSERT INTO district (id, city_id, name) VALUES ('950813', '9508', 'YENGGELO');
INSERT INTO district (id, city_id, name) VALUES ('950814', '9508', 'KILMID');
INSERT INTO district (id, city_id, name) VALUES ('950815', '9508', 'ALAMA');
INSERT INTO district (id, city_id, name) VALUES ('950816', '9508', 'YAL');
INSERT INTO district (id, city_id, name) VALUES ('950817', '9508', 'MAM');
INSERT INTO district (id, city_id, name) VALUES ('950818', '9508', 'DAL');
INSERT INTO district (id, city_id, name) VALUES ('950819', '9508', 'NIRKURI');
INSERT INTO district (id, city_id, name) VALUES ('950820', '9508', 'INIKGAL');
INSERT INTO district (id, city_id, name) VALUES ('950821', '9508', 'INIYE');
INSERT INTO district (id, city_id, name) VALUES ('950822', '9508', 'MBULMU YALMA');
INSERT INTO district (id, city_id, name) VALUES ('950823', '9508', 'MBUA TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('950824', '9508', 'EMBETPEN');
INSERT INTO district (id, city_id, name) VALUES ('950825', '9508', 'KORA');
INSERT INTO district (id, city_id, name) VALUES ('950826', '9508', 'WUSI');
INSERT INTO district (id, city_id, name) VALUES ('950827', '9508', 'PIJA');
INSERT INTO district (id, city_id, name) VALUES ('950828', '9508', 'MOBA');
INSERT INTO district (id, city_id, name) VALUES ('950829', '9508', 'WUTPAGA');
INSERT INTO district (id, city_id, name) VALUES ('950830', '9508', 'NENGGEAGIN');
INSERT INTO district (id, city_id, name) VALUES ('950831', '9508', 'KREPKURI');
INSERT INTO district (id, city_id, name) VALUES ('950832', '9508', 'PASIR PUTIH');

-- 9601 KABUPATEN SORONG -- city 9601 (30 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('960101', '9601', 'MAKBON');
INSERT INTO district (id, city_id, name) VALUES ('960102', '9601', 'BERAUR');
INSERT INTO district (id, city_id, name) VALUES ('960103', '9601', 'SALAWATI');
INSERT INTO district (id, city_id, name) VALUES ('960104', '9601', 'SEGET');
INSERT INTO district (id, city_id, name) VALUES ('960105', '9601', 'AIMAS');
INSERT INTO district (id, city_id, name) VALUES ('960106', '9601', 'KLAMONO');
INSERT INTO district (id, city_id, name) VALUES ('960107', '9601', 'SAYOSA');
INSERT INTO district (id, city_id, name) VALUES ('960108', '9601', 'SEGUN');
INSERT INTO district (id, city_id, name) VALUES ('960109', '9601', 'MAYAMUK');
INSERT INTO district (id, city_id, name) VALUES ('960110', '9601', 'SALAWATI SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960111', '9601', 'KLABOT');
INSERT INTO district (id, city_id, name) VALUES ('960112', '9601', 'KLAWAK');
INSERT INTO district (id, city_id, name) VALUES ('960113', '9601', 'MAUDUS');
INSERT INTO district (id, city_id, name) VALUES ('960114', '9601', 'MARIAT');
INSERT INTO district (id, city_id, name) VALUES ('960115', '9601', 'KLAYILI');
INSERT INTO district (id, city_id, name) VALUES ('960116', '9601', 'KLASO');
INSERT INTO district (id, city_id, name) VALUES ('960117', '9601', 'MOISEGEN');
INSERT INTO district (id, city_id, name) VALUES ('960118', '9601', 'SORONG');
INSERT INTO district (id, city_id, name) VALUES ('960119', '9601', 'BAGUN');
INSERT INTO district (id, city_id, name) VALUES ('960120', '9601', 'WEMAK');
INSERT INTO district (id, city_id, name) VALUES ('960121', '9601', 'SUNOOK');
INSERT INTO district (id, city_id, name) VALUES ('960122', '9601', 'BUK');
INSERT INTO district (id, city_id, name) VALUES ('960123', '9601', 'SAENGKEDUK');
INSERT INTO district (id, city_id, name) VALUES ('960124', '9601', 'MALABOTOM');
INSERT INTO district (id, city_id, name) VALUES ('960125', '9601', 'KONHIR');
INSERT INTO district (id, city_id, name) VALUES ('960126', '9601', 'KLASAFET');
INSERT INTO district (id, city_id, name) VALUES ('960127', '9601', 'HOBARD');
INSERT INTO district (id, city_id, name) VALUES ('960128', '9601', 'SALAWATI TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('960129', '9601', 'BOTAIN');
INSERT INTO district (id, city_id, name) VALUES ('960130', '9601', 'SAYOSA TIMUR');

-- 9602 KABUPATEN SORONG SELATAN -- city 9602 (15 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('960201', '9602', 'TEMINABUAN');
INSERT INTO district (id, city_id, name) VALUES ('960202', '9602', 'INANWATAN');
INSERT INTO district (id, city_id, name) VALUES ('960203', '9602', 'SAWIAT');
INSERT INTO district (id, city_id, name) VALUES ('960204', '9602', 'KOKODA');
INSERT INTO district (id, city_id, name) VALUES ('960205', '9602', 'MOSWAREN');
INSERT INTO district (id, city_id, name) VALUES ('960206', '9602', 'SEREMUK');
INSERT INTO district (id, city_id, name) VALUES ('960207', '9602', 'WAYER');
INSERT INTO district (id, city_id, name) VALUES ('960208', '9602', 'KAIS');
INSERT INTO district (id, city_id, name) VALUES ('960209', '9602', 'KONDA');
INSERT INTO district (id, city_id, name) VALUES ('960210', '9602', 'MATEMANI');
INSERT INTO district (id, city_id, name) VALUES ('960211', '9602', 'KOKODA UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960212', '9602', 'SAIFI');
INSERT INTO district (id, city_id, name) VALUES ('960213', '9602', 'FOKOUR');
INSERT INTO district (id, city_id, name) VALUES ('960214', '9602', 'SALKMA');
INSERT INTO district (id, city_id, name) VALUES ('960215', '9602', 'KAIS DARAT');

-- 9603 KABUPATEN RAJA AMPAT -- city 9603 (24 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('960301', '9603', 'MISOOL (MISOOL UTARA)');
INSERT INTO district (id, city_id, name) VALUES ('960302', '9603', 'WAIGEO UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960303', '9603', 'WAIGEO SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960304', '9603', 'SALAWATI UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960305', '9603', 'AYAU');
INSERT INTO district (id, city_id, name) VALUES ('960306', '9603', 'MISOOL TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960307', '9603', 'WAIGEO BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960308', '9603', 'WAIGEO TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960309', '9603', 'TELUK MAYALIBIT');
INSERT INTO district (id, city_id, name) VALUES ('960310', '9603', 'KOFIAU');
INSERT INTO district (id, city_id, name) VALUES ('960311', '9603', 'MEOS MANSAR');
INSERT INTO district (id, city_id, name) VALUES ('960312', '9603', 'MISOOL SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960313', '9603', 'WARWARBOMI');
INSERT INTO district (id, city_id, name) VALUES ('960314', '9603', 'WAIGEO BARAT KEPULAUAN');
INSERT INTO district (id, city_id, name) VALUES ('960315', '9603', 'MISOOL BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960316', '9603', 'KEPULAUAN SEMBILAN');
INSERT INTO district (id, city_id, name) VALUES ('960317', '9603', 'KOTA WAISAI');
INSERT INTO district (id, city_id, name) VALUES ('960318', '9603', 'TIPLOL MAYALIBIT');
INSERT INTO district (id, city_id, name) VALUES ('960319', '9603', 'BATANTA UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960320', '9603', 'SALAWATI BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960321', '9603', 'SALAWATI TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('960322', '9603', 'SUPNIN');
INSERT INTO district (id, city_id, name) VALUES ('960323', '9603', 'KEPULAUAN AYAU');
INSERT INTO district (id, city_id, name) VALUES ('960324', '9603', 'BATANTA SELATAN');

-- 9604 KABUPATEN TAMBRAUW -- city 9604 (29 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('960401', '9604', 'FEF');
INSERT INTO district (id, city_id, name) VALUES ('960402', '9604', 'MIYAH');
INSERT INTO district (id, city_id, name) VALUES ('960403', '9604', 'YEMBUN');
INSERT INTO district (id, city_id, name) VALUES ('960404', '9604', 'KWOOR');
INSERT INTO district (id, city_id, name) VALUES ('960405', '9604', 'SAUSAPOR');
INSERT INTO district (id, city_id, name) VALUES ('960406', '9604', 'ABUN');
INSERT INTO district (id, city_id, name) VALUES ('960407', '9604', 'SYUJAK');
INSERT INTO district (id, city_id, name) VALUES ('960408', '9604', 'MORAID');
INSERT INTO district (id, city_id, name) VALUES ('960409', '9604', 'KEBAR');
INSERT INTO district (id, city_id, name) VALUES ('960410', '9604', 'AMBERBAKEN');
INSERT INTO district (id, city_id, name) VALUES ('960411', '9604', 'SENOPI');
INSERT INTO district (id, city_id, name) VALUES ('960412', '9604', 'MUBRANI');
INSERT INTO district (id, city_id, name) VALUES ('960413', '9604', 'BIKAR');
INSERT INTO district (id, city_id, name) VALUES ('960414', '9604', 'BAMUSBAMA');
INSERT INTO district (id, city_id, name) VALUES ('960415', '9604', 'ASES');
INSERT INTO district (id, city_id, name) VALUES ('960416', '9604', 'MIYAH SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960417', '9604', 'IRERES');
INSERT INTO district (id, city_id, name) VALUES ('960418', '9604', 'TOBOUW');
INSERT INTO district (id, city_id, name) VALUES ('960419', '9604', 'WILHEM ROUMBOUTS');
INSERT INTO district (id, city_id, name) VALUES ('960420', '9604', 'TINGGOUW');
INSERT INTO district (id, city_id, name) VALUES ('960421', '9604', 'KWESEFO');
INSERT INTO district (id, city_id, name) VALUES ('960422', '9604', 'MAWABUAN');
INSERT INTO district (id, city_id, name) VALUES ('960423', '9604', 'KEBAR TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960424', '9604', 'KEBAR SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960425', '9604', 'MANEKAR');
INSERT INTO district (id, city_id, name) VALUES ('960426', '9604', 'MPUR');
INSERT INTO district (id, city_id, name) VALUES ('960427', '9604', 'AMBERBAKEN BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960428', '9604', 'KASI');
INSERT INTO district (id, city_id, name) VALUES ('960429', '9604', 'SELEMKAI');

-- 9605 KABUPATEN MAYBRAT -- city 9605 (24 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('960501', '9605', 'AIFAT');
INSERT INTO district (id, city_id, name) VALUES ('960502', '9605', 'AIFAT UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960503', '9605', 'AIFAT TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960504', '9605', 'AIFAT SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960505', '9605', 'AITINYO BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960506', '9605', 'AITINYO');
INSERT INTO district (id, city_id, name) VALUES ('960507', '9605', 'AITINYO UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960508', '9605', 'AYAMARU');
INSERT INTO district (id, city_id, name) VALUES ('960509', '9605', 'AYAMARU UTARA');
INSERT INTO district (id, city_id, name) VALUES ('960510', '9605', 'AYAMARU TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960511', '9605', 'MARE');
INSERT INTO district (id, city_id, name) VALUES ('960512', '9605', 'AIFAT TIMUR TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('960513', '9605', 'AIFAT TIMUR JAUH');
INSERT INTO district (id, city_id, name) VALUES ('960514', '9605', 'AIFAT TIMUR SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960515', '9605', 'AYAMARU SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960516', '9605', 'AYAMARU JAYA');
INSERT INTO district (id, city_id, name) VALUES ('960517', '9605', 'AYAMARU SELATAN JAYA');
INSERT INTO district (id, city_id, name) VALUES ('960518', '9605', 'AYAMARU TIMUR SELATAN');
INSERT INTO district (id, city_id, name) VALUES ('960519', '9605', 'AYAMARU UTARA TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('960520', '9605', 'AYAMARU TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('960521', '9605', 'AYAMARU BARAT');
INSERT INTO district (id, city_id, name) VALUES ('960522', '9605', 'AITINYO TENGAH');
INSERT INTO district (id, city_id, name) VALUES ('960523', '9605', 'AITINYO RAYA');
INSERT INTO district (id, city_id, name) VALUES ('960524', '9605', 'MARE SELATAN');

-- 9671 KOTA SORONG -- city 9671 (10 kecamatan)
INSERT INTO district (id, city_id, name) VALUES ('967101', '9671', 'SORONG');
INSERT INTO district (id, city_id, name) VALUES ('967102', '9671', 'SORONG TIMUR');
INSERT INTO district (id, city_id, name) VALUES ('967103', '9671', 'SORONG BARAT');
INSERT INTO district (id, city_id, name) VALUES ('967104', '9671', 'SORONG KEPULAUAN');
INSERT INTO district (id, city_id, name) VALUES ('967105', '9671', 'SORONG UTARA');
INSERT INTO district (id, city_id, name) VALUES ('967106', '9671', 'SORONG MANOI');
INSERT INTO district (id, city_id, name) VALUES ('967107', '9671', 'SORONG KOTA');
INSERT INTO district (id, city_id, name) VALUES ('967108', '9671', 'KLAURUNG');
INSERT INTO district (id, city_id, name) VALUES ('967109', '9671', 'MALAIMSIMSA');
INSERT INTO district (id, city_id, name) VALUES ('967110', '9671', 'MALADUM MES');

