package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

const (
	dbHost     = "192.168.1.49"
	dbPort     = 3306
	dbUser     = "your_db_user"
	dbPassword = "your_db_password"
	dbName     = "your_db_name"
)

// ConnectDB establishes a connection to the MySQL database.
func ConnectDB() (*sql.DB, error) {
	// DSNフォーマット: user:password@tcp(host:port)/dbname?parseTime=true
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// PingDB checks the database connection.
func PingDB(db *sql.DB) error {
	return db.Ping()
}

// QueryStocks executes a SELECT query to retrieve all rows from the stocks table where name matches.
func QueryStocks(db *sql.DB, name string) ([]map[string]interface{}, error) {
	query := "SELECT * FROM stocks WHERE name = ?;"
	rows, err := db.Query(query, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	results := []map[string]interface{}{}
	for rows.Next() {
		columnValues := make([]interface{}, len(columns))
		columnPointers := make([]interface{}, len(columns))
		for i := range columnValues {
			columnPointers[i] = &columnValues[i]
		}
		if err := rows.Scan(columnPointers...); err != nil {
			return nil, err
		}
		rowData := make(map[string]interface{})
		for i, colName := range columns {
			val := columnValues[i]
			if b, ok := val.([]byte); ok {
				rowData[colName] = string(b)
			} else {
				rowData[colName] = val
			}
		}
		results = append(results, rowData)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

// UpsertStock は在庫データを更新または挿入します。
// nameが既に存在する場合はamountを加算し、存在しない場合は新規レコードを作成します。
func UpsertStock(db *sql.DB, name string, amount int) error {
	// 最初にnameが存在するか確認
	var existingAmount int
	var exists bool

	query := "SELECT amount FROM stocks WHERE name = ?;"
	err := db.QueryRow(query, name).Scan(&existingAmount)

	if err != nil {
		if err == sql.ErrNoRows {
			// 該当レコードが存在しない場合は新規挿入
			exists = false
		} else {
			// その他のエラーが発生した場合
			return fmt.Errorf("データ確認中にエラーが発生: %v", err)
		}
	} else {
		exists = true
	}

	// トランザクション開始
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("トランザクション開始エラー: %v", err)
	}
	defer tx.Rollback() // エラー発生時にロールバック

	if exists {
		// 既存レコードの更新
		newAmount := existingAmount + amount
		updateQuery := "UPDATE stocks SET amount = ? WHERE name = ?;"
		_, err = tx.Exec(updateQuery, newAmount, name)
		if err != nil {
			return fmt.Errorf("データ更新エラー: %v", err)
		}
	} else {
		// 新規レコード挿入
		insertQuery := "INSERT INTO stocks (name, amount) VALUES (?, ?);"
		_, err = tx.Exec(insertQuery, name, amount)
		if err != nil {
			return fmt.Errorf("データ挿入エラー: %v", err)
		}
	}

	// トランザクションをコミット
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("トランザクションコミットエラー: %v", err)
	}

	return nil
}
