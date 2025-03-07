package main

import "database/sql"

// MockDB はユニットテストで使用するデータベース操作を実行するためのインターフェースです。
type MockDB interface {
	// 基本的なクエリ実行メソッド
	Exec(query string, args ...interface{}) (int, error)
	Query(query string, args ...interface{}) (MockRows, error)
	QueryRow(query string, args ...interface{}) MockRow

	// トランザクション関連メソッド
	Begin() (MockTx, error)

	// 接続管理メソッド
	Close() error
	Ping() error
}

// MockRows はクエリ結果の行セットをモックするインターフェースです。
type MockRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Close() error
	Columns() ([]string, error)
}

// MockRow は単一行のクエリ結果をモックするインターフェースです。
type MockRow interface {
	Scan(dest ...interface{}) error
}

// MockTx はデータベーストランザクションをモックするインターフェースです。
type MockTx interface {
	Commit() error
	Rollback() error
	Exec(query string, args ...interface{}) (int, error)
	Query(query string, args ...interface{}) (MockRows, error)
	QueryRow(query string, args ...interface{}) MockRow
}

// SQLDBAdapter は標準のdatabase/sql.DBをMockDBインターフェースに適応させるアダプタです。
// 実装が必要な場合に使用します。
type SQLDBAdapter struct {
	DB *sql.DB
}
