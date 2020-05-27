// Copyright 2017 Vector Creations Ltd
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
//
// Modifications copyright (C) 2020 Finogeeks Co., Ltd

package common

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	log "github.com/finogeeks/ligase/skunkworks/log"
	"github.com/lib/pq"
)

// A Transaction is something that can be committed or rolledback.
type Transaction interface {
	// Commit the transaction
	Commit() error
	// Rollback the transaction.
	Rollback() error
}

// EndTransaction ends a transaction.
// If the transaction succeeded then it is committed, otherwise it is rolledback.
func EndTransaction(txn Transaction, succeeded *bool) {
	//last := time.Now()
	if *succeeded {
		txn.Commit() // nolint: errcheck
	} else {
		txn.Rollback() // nolint: errcheck
	}
	//fmt.Printf("------------------------EndTransaction use %v", time.Now().Sub(last))
}

// WithTransaction runs a block of code passing in an SQL transaction
// If the code returns an error or panics then the transactions is rolledback
// Otherwise the transaction is committed.
func WithTransaction(db *sql.DB, fn func(txn *sql.Tx) error) (err error) {
	txn, err := db.Begin()
	if err != nil {
		return
	}
	succeeded := false
	defer EndTransaction(txn, &succeeded)

	err = fn(txn)
	if err != nil {
		return
	}

	succeeded = true
	return
}

// TxStmt wraps an SQL stmt inside an optional transaction.
// If the transaction is nil then it returns the original statement that will
// run outside of a transaction.
// Otherwise returns a copy of the statement that will run inside the transaction.
func TxStmt(transaction *sql.Tx, statement *sql.Stmt) *sql.Stmt {
	if transaction != nil {
		statement = transaction.Stmt(statement)
	}
	return statement
}

// IsUniqueConstraintViolationErr returns true if the error is a postgresql unique_violation error
func IsUniqueConstraintViolationErr(err error) bool {
	pqErr, ok := err.(*pq.Error)
	return ok && pqErr.Code == "23505"
}

// Hooks satisfies the sqlhook.Hooks interface
type Hooks struct{}

// Before hook will print the query with it's args and return the context with the timestamp
func (h *Hooks) Before(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	log.Infof("> %s %q", query, args)
	return context.WithValue(ctx, "begin", time.Now()), nil
}

// After hook will get the timestamp registered on the Before hook and print the elapsed time
func (h *Hooks) Error(ctx context.Context, errstr error, query string, args ...interface{}) {
	log.Fatalf("> %s %v %q", query, errstr, args)
}

// After hook will get the timestamp registered on the Before hook and print the elapsed time
func (h *Hooks) After(ctx context.Context, query string, args ...interface{}) (context.Context, error) {
	begin := ctx.Value("begin").(time.Time)
	log.Infof(". took: %v", time.Since(begin))
	return ctx, nil
}

func CreateDatabase(driver, addr, name string) error {
	db, err := sql.Open(driver, addr)
	if err != nil {
		return err
	}

	dbName := ""
	sqlStr := fmt.Sprintf("SELECT datname FROM pg_catalog.pg_database WHERE datname = '%s'", name)
	db.QueryRow(sqlStr).Scan(&dbName)
	if err == sql.ErrNoRows || dbName == "" {
		sqlStr := fmt.Sprintf("CREATE DATABASE %s", name)
		db.Exec(sqlStr)
		time.Sleep(time.Second * 10) //wait master&slave sync
	}
	db.Close()

	return nil
}
