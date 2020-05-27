// Copyright (C) 2020 Finogeeks Co., Ltd
//
// This program is free software: you can redistribute it and/or  modify
// it under the terms of the GNU Affero General Public License, version 3,
// as published by the Free Software Foundation.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package keydb

import (
	"context"
	"database/sql"
)

const CertSchema = `
CREATE TABLE IF NOT EXISTS keydb_cert (
	root_ca TEXT NOT NULL DEFAULT '',
    server_cert TEXT NOT NULL DEFAULT '',
	server_key TEXT NOT NULL DEFAULT '',
	crl TEXT NOT NULL DEFAULT ''
);
`

const insertRootCASQL = "INSERT INTO keydb_cert (root_ca) VALUES ($1)"
const selectAllCertsSQL = "SELECT * FROM keydb_cert limit 1"
const updateCertSQL = "UPDATE keydb_cert SET server_cert=$1,server_key=$2 WHERE root_ca != ''"
const updateCRLSQL = "UPDATE keydb_cert SET crl=$1 WHERE root_ca != ''"

type certStatements struct {
	insertRootCAStmt   *sql.Stmt
	selectAllCertsStmt *sql.Stmt
	updateCertStmt     *sql.Stmt
	updateCRLStmt      *sql.Stmt
}

func (s *certStatements) prepare(db *sql.DB) (err error) {
	_, err = db.Exec(CertSchema)
	if err != nil {
		return err
	}
	if s.insertRootCAStmt, err = db.Prepare(insertRootCASQL); err != nil {
		return
	}
	if s.selectAllCertsStmt, err = db.Prepare(selectAllCertsSQL); err != nil {
		return
	}
	if s.updateCertStmt, err = db.Prepare(updateCertSQL); err != nil {
		return
	}
	if s.updateCRLStmt, err = db.Prepare(updateCRLSQL); err != nil {
		return
	}
	return
}

func (s *certStatements) insertRootCA(
	ctx context.Context,
	rootCA string,
) error {
	_, err := s.insertRootCAStmt.ExecContext(ctx, rootCA)
	return err
}

func (s *certStatements) selectAllCerts(
	ctx context.Context,
) (string, string, string, string, error) {
	var rootCA, serverCert, serverKey, CRL string
	err := s.selectAllCertsStmt.QueryRowContext(ctx).Scan(&rootCA, &serverCert, &serverKey, &CRL)
	if err != nil {
		return "", "", "", "", err
	}

	return rootCA, serverCert, serverKey, CRL, nil
}

func (s *certStatements) upsertCert(
	ctx context.Context,
	serverCert, serverKey string,
) error {
	_, err := s.updateCertStmt.ExecContext(ctx, serverCert, serverKey)
	return err
}

func (s *certStatements) upsertCRL(
	ctx context.Context,
	CRL string,
) error {
	_, err := s.updateCRLStmt.ExecContext(ctx, CRL)
	return err
}
