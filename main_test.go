package main

import (
	"bytes"
	"database/sql"
	"errors"
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
)

// captureOutput は標準出力をリダイレクトして、関数実行中の出力をキャプチャします。
func captureOutput(f func()) string {
	// 標準出力のバックアップ
	oldStdout := os.Stdout

	// os.Pipe() の作成
	r, w, err := os.Pipe()
	if err != nil {
		panic("os.Pipe() エラー: " + err.Error())
	}

	// 標準出力をパイプの書き込み側に変更
	os.Stdout = w
	defer func() {
		os.Stdout = oldStdout
	}()

	// 指定関数の実行
	f()

	// 書き込み側を閉じる
	w.Close()

	// パイプから出力を読み取る
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		panic("buf.ReadFrom() エラー: " + err.Error())
	}
	r.Close()

	return buf.String()
}

/* =============================
   テストケース：mainProcessの動作
   ============================= */

// TestMainFunctionWithMock はモックDBを使ってmainProcessの動作をテストします
func TestMainFunctionWithMock(t *testing.T) {
	db, mock, err := setupMockDB(t)
	assert.NoError(t, err, "モックDBのセットアップに成功するべき")
	defer db.Close()

	// Ping成功のモック設定
	mock.ExpectPing()

	// 「apple」検索クエリと結果のモック設定
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("apple").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount"}).
			AddRow(1, "apple", 100))

	// UpsertStockのモック設定：既存データ確認、UPDATE実行
	mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
		WithArgs("apple").
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(100))
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE stocks SET amount = \? WHERE name = \?;`).
		WithArgs(300, "apple").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	// mainProcessの実行と出力キャプチャ
	output := captureOutput(func() {
		err := mainProcess(db, "apple", 200)
		assert.NoError(t, err, "mainProcessは成功するべき")
	})

	// モックの期待通りにクエリが実行されたか確認
	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")

	// 出力内容の検証
	expectedOutputs := []string{
		"全ての行:", "apple", "100",
		"在庫データが更新されました",
	}
	for _, expected := range expectedOutputs {
		assert.Contains(t, output, expected, "出力に '%s' が含まれるべき", expected)
	}
}

// TestMainProcess_ConnectionError はDB接続エラー時の動作をテストします
func TestMainProcess_ConnectionError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	assert.NoError(t, err, "モックDBの作成に成功するべき")
	defer db.Close()

	if mock == nil {
		t.Fatal("mockオブジェクトがnilです")
	}

	// Pingでエラーを返す設定
	mock.ExpectPing().WillReturnError(errors.New("接続エラー"))

	// mainProcessの実行
	err = mainProcess(db, "apple", 200)
	assert.Error(t, err, "DB接続確認エラーが発生するべき")
	assert.Contains(t, err.Error(), "DB接続確認に失敗", "適切なエラーメッセージを含むべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")
}

// TestMainProcess_QueryError はPing以降のクエリエラー時の動作をテストします
func TestMainProcess_QueryError(t *testing.T) {
	db, mock, err := setupMockDB(t)
	assert.NoError(t, err, "モックDBのセットアップに成功するべき")
	defer db.Close()

	mock.ExpectPing()

	// 「apple」検索クエリでエラーを返す設定
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("apple").
		WillReturnError(errors.New("クエリエラー"))

	err = mainProcess(db, "apple", 200)
	assert.Error(t, err, "クエリエラーが発生するべき")
	assert.Contains(t, err.Error(), "クエリ実行に失敗", "適切なエラーメッセージを含むべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")
}

// TestMainProcess_UpsertError はデータ更新エラー時の動作をテストします
func TestMainProcess_UpsertError(t *testing.T) {
	db, mock, err := setupMockDB(t)
	assert.NoError(t, err, "モックDBのセットアップに成功するべき")
	defer db.Close()

	mock.ExpectPing()

	// 「apple」検索クエリで結果取得
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("apple").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount"}).
			AddRow(1, "apple", 100))

	// Upsert時のSELECTでエラー発生をモック
	mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
		WithArgs("apple").
		WillReturnError(errors.New("データ取得エラー"))

	err = mainProcess(db, "apple", 200)
	assert.Error(t, err, "データ更新エラーが発生するべき")
	assert.Contains(t, err.Error(), "在庫更新エラー", "適切なエラーメッセージを含むべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")
}

// TestMainProcess_EmptyResult は検索結果が空の場合の動作をテストします
func TestMainProcess_EmptyResult(t *testing.T) {
	db, mock, err := setupMockDB(t)
	assert.NoError(t, err, "モックDBのセットアップに成功するべき")
	defer db.Close()

	mock.ExpectPing()

	// 空の検索結果を返すモック設定
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("nonexistent").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount"}))

	// 新規商品の挿入処理用モック設定
	mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
		WithArgs("nonexistent").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO stocks \(name, amount\) VALUES \(\?, \?\);`).
		WithArgs("nonexistent", 50).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	output := captureOutput(func() {
		err := mainProcess(db, "nonexistent", 50)
		assert.NoError(t, err, "mainProcessは成功するべき")
	})

	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")
	assert.Contains(t, output, "結果が見つかりませんでした", "該当メッセージが出力されるべき")
	assert.Contains(t, output, "在庫データが更新されました", "更新成功メッセージが含まれるべき")
}

// TestMainProcess_NewItemInsert は新規商品の挿入をテストします
func TestMainProcess_NewItemInsert(t *testing.T) {
	db, mock, err := setupMockDB(t)
	assert.NoError(t, err, "モックDBのセットアップに成功するべき")
	defer db.Close()

	mock.ExpectPing()

	// 「banana」の検索クエリでデータが存在しない状態
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("banana").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount"}))

	// 新規商品挿入のためのモック設定
	mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
		WithArgs("banana").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec(`INSERT INTO stocks \(name, amount\) VALUES \(\?, \?\);`).
		WithArgs("banana", 50).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	output := captureOutput(func() {
		err := mainProcess(db, "banana", 50)
		assert.NoError(t, err, "mainProcessは成功するべき")
	})

	assert.NoError(t, mock.ExpectationsWereMet(), "期待されたすべてのクエリが実行されるべき")
	assert.Contains(t, output, "結果が見つかりませんでした", "該当メッセージが出力されるべき")
	assert.Contains(t, output, "在庫データが更新されました", "更新成功メッセージが含まれるべき")
}

/* =============================
   テストケース：ConnectDBの動作
   ============================= */

// TestConnectDBWithMock は ConnectDB 関数のモックテストです
func TestConnectDBWithMock(t *testing.T) {
	originalOpenDBFunc := openDBFunc
	defer func() { openDBFunc = originalOpenDBFunc }()

	// 接続成功ケース
	t.Run("接続成功", func(t *testing.T) {
		openDBFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
			assert.Equal(t, "mysql", driverName, "ドライバー名はmysqlであるべき")
			assert.Contains(t, dataSourceName, "your_db_user:your_db_password", "接続文字列にユーザー情報が含まれるべき")
			assert.Contains(t, dataSourceName, "@tcp(", "接続文字列にTCP接続情報が含まれるべき")

			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock作成エラー: %v", err)
			}
			return db, nil
		}

		db, err := ConnectDB()
		assert.NoError(t, err, "接続は成功するべき")
		assert.NotNil(t, db, "DBはnilであるべきではない")
	})

	// 接続失敗ケース
	t.Run("接続失敗", func(t *testing.T) {
		openDBFunc = func(driverName, dataSourceName string) (*sql.DB, error) {
			return nil, errors.New("接続エラー")
		}

		db, err := ConnectDB()
		assert.Error(t, err, "エラーが返されるべき")
		assert.Nil(t, db, "DBはnilであるべき")
		assert.Contains(t, err.Error(), "接続エラー", "エラーメッセージは '接続エラー' を含むべき")
	})
}
