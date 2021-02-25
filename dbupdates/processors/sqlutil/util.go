package sqlutil

import "database/sql"

// A Transaction is something that can be committed or rolledback.
type Transaction interface {
	// Commit the transaction
	Commit() error
	// Rollback the transaction.
	Rollback() error
}

// EndTransaction ends a transaction.
// If the transaction succeeded then it is committed, otherwise it is rolledback.
func EndTransaction(txn0, txn1 Transaction, succeeded *bool) {
	//last := time.Now()
	if *succeeded {
		txn0.Commit() // nolint: errcheck
		txn1.Commit()
	} else {
		txn0.Rollback() // nolint: errcheck
		txn1.Rollback()
	}
	//fmt.Printf("------------------------EndTransaction use %v", time.Now().Sub(last))
}

// WithTransaction runs a block of code passing in an SQL transaction
// If the code returns an error or panics then the transactions is rolledback
// Otherwise the transaction is committed.
func WithTransaction(db *sql.DB, fn func(txn0, txn1 *sql.Tx) error) (err error) {
	txn0, err := db.Begin()
	if err != nil {
		return
	}
	txn1, err := db.Begin()
	if err != nil {
		txn0.Rollback()
		return
	}
	succeeded := false
	defer EndTransaction(txn0, txn1, &succeeded)

	err = fn(txn0, txn1)
	if err != nil {
		return
	}

	succeeded = true
	return
}
