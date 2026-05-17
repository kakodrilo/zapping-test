package db

import (
    "database/sql"
    "fmt"
    "os"
    _ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func InitDB() error {
    // DSN: usuario:password@tcp(host:puerto)/nombre_bd
    dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_unicode_ci&parseTime=true",
        os.Getenv("MYSQL_USER"),
        os.Getenv("MYSQL_PASSWORD"),
        os.Getenv("DB_HOST"),
        os.Getenv("DB_PORT"),
        os.Getenv("MYSQL_DATABASE"),
    )

    var err error
    DB, err = sql.Open("mysql", dsn)
    if err != nil {
        return err
    }
    return DB.Ping()
}