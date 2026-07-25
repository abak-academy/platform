// Canonical jenjang values.
//
// Registrants are not all school pupils — mahasiswa and the general public sign
// up too — so the list spans school levels and higher education. There is no DB
// constraint on users.jenjang; the backend only checks jenjang against a
// school's school_types when a school is actually linked, so this list is the
// single source of truth for what the UI offers.
//
// It previously existed as three divergent copies (student profile, admin
// register, and the exam participant filter), which is why a student could hold
// a jenjang the profile form could not set.
export const JENJANG_OPTIONS = [
  "SD",
  "SMP",
  "SMA",
  "MA",
  "SMK",
  "PKBM",
  "LKP",
  "Kursus",
  "D1",
  "D2",
  "D3",
  "S1",
  "S2",
];
