package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	// テスト用Docker設定
	testDBName     = "test_db"
	testDBUser     = "test_user"
	testDBPassword = "test_password"
	testDBPort     = "3307" // ホストとのポート競合を避けるため
	testDBHost     = "localhost"

	// コンテナ名
	containerName = "mysql_integration_test"

	// テスト用テーブル作成SQL
	createTableSQL = `
CREATE TABLE IF NOT EXISTS stocks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    amount INT NOT NULL,
    UNIQUE(name)
);`
)

// setupIntegrationTest はMySQL Dockerコンテナを起動し、テスト用DBを準備します
func setupIntegrationTest(t *testing.T) (*sql.DB, func()) {
	// インテグレーションテストをスキップする環境変数のチェック
	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("環境変数SKIP_INTEGRATIONが設定されているため、インテグレーションテストをスキップします")
	}

	// 既存のコンテナを停止・削除（もし存在すれば）
	exec.Command("docker", "rm", "-f", containerName).Run()

	// MySQLコンテナを起動
	dockerCmd := exec.Command(
		"docker", "run", "-d",
		"--name", containerName,
		"-e", "MYSQL_ROOT_PASSWORD=root",
		"-e", fmt.Sprintf("MYSQL_DATABASE=%s", testDBName),
		"-e", fmt.Sprintf("MYSQL_USER=%s", testDBUser),
		"-e", fmt.Sprintf("MYSQL_PASSWORD=%s", testDBPassword),
		"-p", fmt.Sprintf("%s:3306", testDBPort),
		"mysql:8.0",
		"--character-set-server=utf8mb4",
		"--collation-server=utf8mb4_unicode_ci",
	)

	output, err := dockerCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Dockerコンテナの起動に失敗: %v, 出力: %s", err, output)
	}

	cleanup := func() {
		exec.Command("docker", "rm", "-f", containerName).Run()
	}

	// データベースが準備できるまで待機
	var db *sql.DB
	var connectErr error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s",
		testDBUser, testDBPassword, "192.168.1.78", testDBPort, testDBName)

	// ログにDSNを出力（パスワードは隠してもよい）
	t.Logf("接続DSN: %s", dsn)

	// MySQLコンテナの状態確認
	// waitForMySQLReady(t, 30)

	// 最大60秒待機に延長
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	t.Log("MySQLコンテナに接続を試行中...")
	ticker := time.NewTicker(2 * time.Second) // 2秒ごとに試行
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			db, connectErr = sql.Open("mysql", dsn)
			if connectErr == nil {
				t.Log("Open成功、Ping試行中...")
				if pingErr := db.Ping(); pingErr == nil {
					t.Log("MySQLコンテナの準備完了")
					goto connected
				} else {
					t.Logf("Pingエラー: %v", pingErr)
					db.Close()
				}
			} else {
				t.Logf("接続エラー: %v", connectErr)
			}
		case <-ctx.Done():
			cleanup()
			t.Fatalf("タイムアウト: MySQLコンテナに接続できません。最後のエラー: %v", connectErr)
		}
	}

connected:
	// テーブル作成
	_, err = db.Exec(createTableSQL)
	if err != nil {
		cleanup()
		t.Fatalf("テーブル作成エラー: %v", err)
	}

	// テスト用データ投入
	_, err = db.Exec("INSERT INTO stocks (name, amount) VALUES (?, ?)", "apple", 100)
	if err != nil {
		cleanup()
		t.Fatalf("テストデータ挿入エラー: %v", err)
	}

	return db, cleanup
}

// TestIntegrationDBConnection は実際のMySQLコンテナへの接続をテストします
func TestIntegrationDBConnection(t *testing.T) {
	// インテグレーションテストのセットアップ
	db, cleanup := setupIntegrationTest(t)
	defer cleanup() // テスト終了時にコンテナをクリーンアップ

	// 一時的にopenDBFuncを保存して、テスト後に復元
	originalOpenDBFunc := openDBFunc

	// openDBFuncを上書きして実際の接続を許可
	openDBFunc = sql.Open

	defer func() { openDBFunc = originalOpenDBFunc }()

	// テスト用のDBホスト・認証情報を設定
	origHost, origPort := dbHost, dbPort
	origUser, origPass := dbUser, dbPassword
	origDbName := dbName

	// テスト用の設定に一時的に変更
	dbHost = testDBHost
	dbPort = 3307 // 文字列からintへの変換
	dbUser = testDBUser
	dbPassword = testDBPassword
	dbName = testDBName

	// 終了時に元の設定に戻す
	defer func() {
		dbHost, dbPort = origHost, origPort
		dbUser, origPass = origUser, origPass
		dbName = origDbName
	}()

	t.Run("実DB接続テスト", func(t *testing.T) {
		// ConnectDB関数を使って接続
		testDB, err := ConnectDB()
		if err != nil {
			t.Fatalf("DB接続エラー: %v", err)
		}
		defer testDB.Close()

		// Ping確認
		err = PingDB(testDB)
		if err != nil {
			t.Fatalf("DB Pingエラー: %v", err)
		}

		t.Log("実DBへの接続とPing成功")
	})

	t.Run("実DBでのクエリテスト", func(t *testing.T) {
		// テストデータの検索
		results, err := QueryStocks(db, "apple")
		if err != nil {
			t.Fatalf("QueryStocksエラー: %v", err)
		}

		// 結果の検証
		if len(results) != 1 {
			t.Fatalf("期待される結果数: 1, 実際: %d", len(results))
		}

		if results[0]["name"] != "apple" {
			t.Errorf("期待される名前: apple, 実際: %v", results[0]["name"])
		}

		if results[0]["amount"] != int64(100) {
			t.Errorf("期待される数量: 100, 実際: %v", results[0]["amount"])
		}

		t.Log("実DBでのクエリテスト成功")
	})

	t.Run("実DBでのUpsertテスト", func(t *testing.T) {
		// 新規データのUpsert
		err := UpsertStock(db, "banana", 50)
		if err != nil {
			t.Fatalf("UpsertStockエラー (INSERT): %v", err)
		}

		// 既存データの更新
		err = UpsertStock(db, "apple", 200)
		if err != nil {
			t.Fatalf("UpsertStockエラー (UPDATE): %v", err)
		}

		// 変更を確認
		results, err := QueryStocks(db, "apple")
		if err != nil {
			t.Fatalf("更新後のQueryStocksエラー: %v", err)
		}

		if len(results) != 1 || results[0]["amount"] != int64(300) {
			t.Errorf("期待されるappleの数量: 300, 実際: %v", results[0]["amount"])
		}

		t.Log("実DBでのUpsertテスト成功")
	})
}

// waitForMySQLReady の代替として使用
func waitForMySQLReady(t *testing.T, maxRetries int) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?timeout=5s",
		testDBUser, testDBPassword, testDBHost, testDBPort, testDBName)

	t.Logf("MySQL接続を待機中...")

	for i := 0; i < maxRetries; i++ {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			db.Close()

			if err == nil {
				t.Logf("MySQL接続成功（%d/%d）", i+1, maxRetries)
				return
			}
			t.Logf("MySQL Pingエラー (%d/%d): %v", i+1, maxRetries, err)
		} else {
			t.Logf("MySQL接続エラー (%d/%d): %v", i+1, maxRetries, err)
		}

		time.Sleep(2 * time.Second)
	}

	t.Logf("MySQLへの接続タイムアウト")
}
