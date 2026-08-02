package repository

// Compile-time check that Repository has outbox methods.
var _ = (*Repository).InsertOutboxEvent
var _ = (*Repository).ClaimOutboxEvents
var _ = (*Repository).MarkOutboxProcessed
