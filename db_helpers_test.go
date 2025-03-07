package main

import (
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// setupMockDB はモックDBをセットアップし、テスト用のモックDBとmockオブジェクトを返します
func setupMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock, error) {
	// オリジナルの関数を保存
	originalOpenDBFunc := openDBFunc

	// テスト終了時に復元
	t.Cleanup(func() {
		openDBFunc = originalOpenDBFunc
	})

	// モックDBを作成
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmockの初期化エラー: %v", err)
	}

	// グローバルな関数をモック化
	openDBFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}

	return db, mock, nil
}

// verifyExpectations はすべての期待されたクエリが実行されたかを検証します
func verifyExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("期待されたクエリが実行されませんでした: %v", err)
	}
}

// setupTransaction はモックでトランザクションの開始とコミットを期待するよう設定します
// func setupTransaction(mock sqlmock.Sqlmock) {
// 	mock.ExpectBegin()
// 	mock.ExpectCommit()
// }

// setupRollbackTransaction はモックでトランザクションの開始とロールバックを期待するよう設定します
// func setupRollbackTransaction(mock sqlmock.Sqlmock) {
// 	mock.ExpectBegin()
// 	mock.ExpectRollback()
// }
