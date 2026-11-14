package repository

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// TransactionManager runs fn inside a real database transaction. The
// transaction is stored in the context, and every repository obtains its
// connection through DBFrom(ctx, db), so all repository calls made with the
// ctx passed to fn share that one transaction.
type TransactionManager struct {
	db *gorm.DB
}

func NewTransactionManager(db *gorm.DB) *TransactionManager {
	return &TransactionManager{db: db}
}

// WithTx joins an existing transaction when ctx already carries one, so nested
// WithTx calls don't open a second transaction.
func (tm *TransactionManager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return tm.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// DBFrom returns the transaction stored in ctx if there is one, otherwise the
// fallback connection. Every repository method must use this instead of
// touching r.db directly, otherwise its writes escape the transaction.
func DBFrom(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if tx, ok := ctx.Value(txKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}
