package main

import (
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// setupUpsertMock はUpsertStockのテスト用にモックを設定します
func setupUpsertMock(t *testing.T, name string, existingAmount *int, addAmount int) (*sql.DB, sqlmock.Sqlmock) {
	db, mock, _ := setupMockDB(t)

	if existingAmount == nil {
		// 存在しない商品（INSERT）
		mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
			WithArgs(name).
			WillReturnError(sql.ErrNoRows)

		// トランザクション開始
		mock.ExpectBegin()

		// ここがポイント：正確なSQLクエリ文字列を指定
		mock.ExpectExec(`INSERT INTO stocks \(name, amount\) VALUES \(\?, \?\);`).
			WithArgs(name, addAmount).
			WillReturnResult(sqlmock.NewResult(1, 1))

		// コミット
		mock.ExpectCommit()
	} else {
		// 既存商品（UPDATE）
		newAmount := *existingAmount + addAmount

		mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
			WithArgs(name).
			WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(*existingAmount))

		// トランザクション開始
		mock.ExpectBegin()

		// ここもポイント：正確なSQLクエリ文字列を指定
		mock.ExpectExec(`UPDATE stocks SET amount = \? WHERE name = \?;`).
			WithArgs(newAmount, name).
			WillReturnResult(sqlmock.NewResult(0, 1))

		// コミット
		mock.ExpectCommit()
	}

	return db, mock
}

func TestUpsertStock(t *testing.T) {
	tests := []struct {
		name      string
		stockName string
		amount    int
		existing  *int // nilの場合は存在しない商品、値がある場合は既存の商品と数量
	}{
		{
			name:      "存在しない商品 → INSERT",
			stockName: "banana",
			amount:    50,
			existing:  nil, // 存在しない商品
		},
		{
			name:      "既存商品 → UPDATE",
			stockName: "apple",
			amount:    50,
			existing:  func() *int { val := 100; return &val }(), // 既存の数量
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// 共通ヘルパー関数を使用してモックをセットアップ
			db, mock, _ := setupMockDB(t) // 連鎖的にopenDBFuncもモック化される

			// 以下、必要なモック設定...
			if tc.existing == nil {
				// 存在しない商品（INSERT）のテストパターン設定
				mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
					WithArgs(tc.stockName).
					WillReturnError(sql.ErrNoRows)

				mock.ExpectBegin()
				mock.ExpectExec(`INSERT INTO stocks \(name, amount\) VALUES \(\?, \?\);`).
					WithArgs(tc.stockName, tc.amount).
					WillReturnResult(sqlmock.NewResult(1, 1))
				mock.ExpectCommit()
			} else {
				// 既存商品（UPDATE）のテストパターン設定
				newAmount := *tc.existing + tc.amount

				mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
					WithArgs(tc.stockName).
					WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(*tc.existing))

				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE stocks SET amount = \? WHERE name = \?;`).
					WithArgs(newAmount, tc.stockName).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectCommit()
			}

			// UpsertStock関数を実行 - この時点でdb接続はモック化されている
			err := UpsertStock(db, tc.stockName, tc.amount)
			if err != nil {
				t.Fatalf("予期せぬエラー: %v", err)
			}

			// 期待通りのSQL実行を検証
			verifyExpectations(t, mock)
		})
	}
}

// トランザクションエラーのテスト
func TestUpsertStock_TransactionErrors(t *testing.T) {
	testCases := []struct {
		name        string
		itemName    string
		amount      int
		setupMock   func(mock sqlmock.Sqlmock)
		expectedErr string
	}{
		{
			name:     "トランザクション開始エラー",
			itemName: "apple",
			amount:   50,
			setupMock: func(mock sqlmock.Sqlmock) {
				// SELECTは成功
				mock.ExpectQuery(`SELECT\s+amount\s+FROM\s+stocks\s+WHERE\s+name\s*=\s*\?`).
					WithArgs("apple").
					WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(100))
				// Beginでエラー
				mock.ExpectBegin().WillReturnError(errors.New("begin transaction error"))
			},
			expectedErr: "トランザクション開始エラー",
		},
		{
			name:     "更新実行エラー",
			itemName: "apple",
			amount:   50,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT\s+amount\s+FROM\s+stocks\s+WHERE\s+name\s*=\s*\?`).
					WithArgs("apple").
					WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(100))
				mock.ExpectBegin()
				// UPDATE実行でエラー
				mock.ExpectExec(`UPDATE\s+stocks\s+SET\s+amount\s*=\s*\?\s+WHERE\s+name\s*=\s*\?`).
					WithArgs(150, "apple").
					WillReturnError(errors.New("update execution error"))
				mock.ExpectRollback()
			},
			expectedErr: "データ更新エラー",
		},
		{
			name:     "挿入実行エラー",
			itemName: "new_item",
			amount:   50,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT\s+amount\s+FROM\s+stocks\s+WHERE\s+name\s*=\s*\?`).
					WithArgs("new_item").
					WillReturnError(sql.ErrNoRows)
				mock.ExpectBegin()
				// INSERT実行でエラー
				mock.ExpectExec(`INSERT INTO stocks\s*\(name,\s*amount\)\s*VALUES\s*\(\?,\s*\?\)`).
					WithArgs("new_item", 50).
					WillReturnError(errors.New("insert execution error"))
				mock.ExpectRollback()
			},
			expectedErr: "データ挿入エラー",
		},
		{
			name:     "コミットエラー",
			itemName: "apple",
			amount:   50,
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT\s+amount\s+FROM\s+stocks\s+WHERE\s+name\s*=\s*\?`).
					WithArgs("apple").
					WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(100))
				mock.ExpectBegin()
				mock.ExpectExec(`UPDATE\s+stocks\s+SET\s+amount\s*=\s*\?\s+WHERE\s+name\s*=\s*\?`).
					WithArgs(150, "apple").
					WillReturnResult(sqlmock.NewResult(0, 1))
				// コミットでエラー
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
			},
			expectedErr: "トランザクションコミットエラー",
		},
	}

	for _, tc := range testCases {
		tc := tc // ローカル変数に束縛
		t.Run(tc.name, func(t *testing.T) {
			// モックDBの設定（setupMockDBは共通のヘルパー関数とする）
			db, mock, err := setupMockDB(t)
			if err != nil {
				t.Fatalf("sqlmockの初期化エラー: %v", err)
			}
			defer db.Close()

			// テスト固有のモック設定を実行
			tc.setupMock(mock)

			// UpsertStock関数を実行
			err = UpsertStock(db, tc.itemName, tc.amount)
			if err == nil {
				t.Fatal("エラーを期待していましたが、nilが返されました")
			}

			// エラーメッセージに期待する文字列が含まれているかシンプルに検証
			if !strings.Contains(err.Error(), tc.expectedErr) {
				t.Fatalf("エラーメッセージに'%s'が含まれることを期待していましたが、'%s'が返されました",
					tc.expectedErr, err.Error())
			}

			// モックの期待がすべて満たされたか検証
			verifyExpectations(t, mock)
		})
	}
}
