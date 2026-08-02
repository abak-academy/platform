package repository

// Compile-time check that Repository has the promo-code methods. This replaces a test
// that only referenced them and asserted nothing.
var _ = (*Repository).GetPromoByCode
var _ = (*Repository).CreatePromoCode
var _ = (*Repository).UpdatePromoCode
var _ = (*Repository).DeletePromoCode
var _ = (*Repository).ListPromoCodes
var _ = (*Repository).IncrementPromoUses
