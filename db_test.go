package main

import (
	"database/sql"
	"errors"
	"strings"
)

// テストパッケージの初期化時に実行される
func init() {
	// テスト実行時は必ず実データベース接続をブロックするためのフック
	// テストケースごとのモック設定より先に実行される
	openDBFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
		// テスト実行中に実際のデータベースへの接続を試みた場合にエラー
		// 実際のDSNを検知して警告
		if strings.Contains(dataSourceName, dbHost) || strings.Contains(dataSourceName, dbName) {
			return nil, errors.New("テスト中に実際のデータベース接続が試行されました。これは許可されていません")
		}
		// このエラーは通常は発生しない（モックが適切に設定されるため）
		return nil, errors.New("テスト中の予期しないDB接続要求")
	}
}
