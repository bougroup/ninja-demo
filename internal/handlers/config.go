package handlers

import (
	"database/sql"

	"github.com/bernardoko/ninja-demo/internal/db"
)

func getConfig(conn *sql.DB, key string) (string, error) { return db.GetConfig(conn, key) }
func setConfig(conn *sql.DB, key, value string) error    { return db.SetConfig(conn, key, value) }
