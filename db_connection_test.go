package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// withMockOpenDBFunc はテスト用にopenDBFuncをモック化し、テスト終了後に元の関数を復元します
func withMockOpenDBFunc(t *testing.T, mockFunc func(driverName, dataSourceName string) (*sql.DB, error), testFunc func()) {
	original := openDBFunc
	defer func() { openDBFunc = original }()
	openDBFunc = mockFunc
	testFunc()
}

func TestConnectDB(t *testing.T) {
	tests := []struct {
		name         string
		mockFunc     func(driverName, dataSourceName string) (*sql.DB, error)
		expectError  bool
		errorMatcher func(error) bool
	}{
		{
			name: "成功パターン",
			mockFunc: func(driverName, dataSourceName string) (*sql.DB, error) {
				// driverName の検証
				if driverName != "mysql" {
					return nil, errors.New("unexpected driver")
				}
				// 簡易的なDSNチェック
				if dataSourceName == "" {
					return nil, errors.New("DSN is empty")
				}
				// 実際の接続は行わず、sql.Openで作成したDBを返す
				db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/testdb")
				if err != nil {
					return nil, err
				}
				return db, nil
			},
			expectError: false,
		},
		{
			name: "エラーパターン",
			mockFunc: func(driverName, dataSourceName string) (*sql.DB, error) {
				return nil, errors.New("simulated connection error")
			},
			expectError: true,
			errorMatcher: func(err error) bool {
				return errors.Is(err, errors.New("simulated connection error"))
			},
		},
	}

	for _, tc := range tests {
		tc := tc // ローカルスコープに束縛
		t.Run(tc.name, func(t *testing.T) {
			withMockOpenDBFunc(t, tc.mockFunc, func() {
				db, err := ConnectDB()
				if tc.expectError {
					if err == nil {
						t.Fatal("エラーが発生するはずですが、nilが返されました")
					}
					// ここでは文字列比較ではなく errors.Is を使用する例（※単純な比較なので、実際はラップされた場合にも対応）
					if err.Error() != "simulated connection error" {
						t.Fatalf("期待するエラー: %v, 実際のエラー: %v", "simulated connection error", err)
					}
				} else {
					if err != nil {
						t.Fatalf("予期せぬエラーが発生: %v", err)
					}
					if db == nil {
						t.Fatal("DB接続がnilです")
					}
				}
			})
		})
	}
}

func TestPingDB(t *testing.T) {
	t.Run("DBPing成功", func(t *testing.T) {
		// sqlmockを使用してモックDB接続を作成
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmockの初期化エラー: %v", err)
		}
		defer db.Close()

		// Pingが成功することを期待
		mock.ExpectPing()

		// PingDB関数を実行
		err = PingDB(db)

		// エラーがないことを検証
		if err != nil {
			t.Fatalf("予期せぬエラーが発生: %v", err)
		}

		// すべての期待されたアクションが実行されたか確認
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("期待されたアクションが実行されませんでした: %v", err)
		}
	})

	t.Run("DBPingエラー", func(t *testing.T) {
		// sqlmockを使用してモックDB接続を作成 (MonitorPingsを有効にする)
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		if err != nil {
			t.Fatalf("sqlmockの初期化エラー: %v", err)
		}
		defer db.Close()

		// 期待されるエラー
		expectedErr := errors.New("simulated ping error")

		// Pingがエラーを返すことを期待
		pingExpectation := mock.ExpectPing()
		if pingExpectation == nil {
			t.Fatal("ExpectPing()がnilを返しました")
		}
		pingExpectation.WillReturnError(expectedErr)

		// PingDB関数を実行
		err = PingDB(db)

		// 期待されるエラーが返されることを検証
		if err == nil {
			t.Fatal("エラーを期待しましたが、nilが返されました")
		}
		if err.Error() != expectedErr.Error() {
			t.Fatalf("予期されるエラー: '%v', 返されたエラー: '%v'", expectedErr, err)
		}

		// すべての期待されたアクションが実行されたか確認
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("期待されたアクションが実行されませんでした: %v", err)
		}
	})
}
