package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/jmoiron/sqlx"
)

func runTransaction(db *sqlx.DB, runner func(*sqlx.Tx) error) error {
	var (
		tx  *sqlx.Tx
		err error
	)

	if tx, err = db.Beginx(); err != nil {
		return err
	}

	if err = runner(tx); err != nil {
		logrus.Warnf("db: failure during transaction: %v", err)
		if err2 := tx.Rollback(); err2 != nil {
			logrus.Warnf("db: transaction rollback: %v", err2)
		}
		return err
	}

	return tx.Commit()
}
