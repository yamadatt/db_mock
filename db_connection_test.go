package main

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert" // 追加
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
		errorMessage string
	}{
		{
			name: "成功パターン",
			mockFunc: func(driverName, dataSourceName string) (*sql.DB, error) {
				// driverName の検証
				assert.Equal(t, "mysql", driverName, "ドライバ名はmysqlであるべき")
				// 簡易的なDSNチェック
				assert.NotEmpty(t, dataSourceName, "DSNは空であってはならない")
				// 実際の接続は行わず、sql.Openで作成したDBを返す
				db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/testdb")
				return db, err
			},
			expectError: false,
		},
		{
			name: "エラーパターン",
			mockFunc: func(driverName, dataSourceName string) (*sql.DB, error) {
				return nil, errors.New("simulated connection error")
			},
			expectError:  true,
			errorMessage: "simulated connection error",
		},
	}

	for _, tc := range tests {
		tc := tc // ローカルスコープに束縛
		t.Run(tc.name, func(t *testing.T) {
			withMockOpenDBFunc(t, tc.mockFunc, func() {
				db, err := ConnectDB()
				if tc.expectError {
					assert.Error(t, err, "エラーが発生するべき")
					assert.Equal(t, tc.errorMessage, err.Error(), "エラーメッセージが一致するべき")
					assert.Nil(t, db, "エラー時はDBがnilであるべき")
				} else {
					assert.NoError(t, err, "エラーが発生すべきでない")
					assert.NotNil(t, db, "DBはnilであるべきでない")
				}
			})
		})
	}
}

func TestPingDB(t *testing.T) {
	t.Run("DBPing成功", func(t *testing.T) {
		// sqlmockを使用してモックDB接続を作成
		db, mock, err := sqlmock.New()
		assert.NoError(t, err, "sqlmockの初期化に成功するべき")
		defer db.Close()

		// Pingが成功することを期待
		mock.ExpectPing()

		// PingDB関数を実行
		err = PingDB(db)

		// エラーがないことを検証
		assert.NoError(t, err, "PingDBは成功するべき")

		// すべての期待されたアクションが実行されたか確認
		assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されたアクションが実行されるべき")
	})

	t.Run("DBPingエラー", func(t *testing.T) {
		// sqlmockを使用してモックDB接続を作成 (MonitorPingsを有効にする)
		db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
		assert.NoError(t, err, "sqlmockの初期化に成功するべき")
		defer db.Close()

		// 期待されるエラー
		expectedErr := errors.New("simulated ping error")

		// Pingがエラーを返すことを期待
		pingExpectation := mock.ExpectPing()
		assert.NotNil(t, pingExpectation, "ExpectPing()はnilを返すべきでない")
		pingExpectation.WillReturnError(expectedErr)

		// PingDB関数を実行
		err = PingDB(db)

		// 期待されるエラーが返されることを検証
		assert.Error(t, err, "エラーが返されるべき")
		assert.Equal(t, expectedErr.Error(), err.Error(), "期待されるエラーメッセージと一致するべき")

		// すべての期待されたアクションが実行されたか確認
		assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されたアクションが実行されるべき")
	})
}
