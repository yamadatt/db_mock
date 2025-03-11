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

// removeContainer は指定したコンテナを削除します。
func removeContainer(t *testing.T) {
	if err := exec.Command("docker", "rm", "-f", containerName).Run(); err != nil {
		t.Logf("コンテナ削除に失敗（既に存在しない可能性あり）: %v", err)
	}
}

// startDockerContainer はMySQLコンテナを起動します。
func startDockerContainer(t *testing.T) {
	// 既存のコンテナを削除
	removeContainer(t)

	cmd := exec.Command(
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
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Dockerコンテナの起動に失敗: %v, 出力: %s", err, output)
	}
}

// waitForMySQL はMySQLコンテナへの接続が可能になるまで待機し、DB接続を返します。
func waitForMySQL(t *testing.T, dsn string) *sql.DB {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	var db *sql.DB
	var connectErr error

	t.Log("MySQLコンテナに接続を試行中...")
	for {
		select {
		case <-ticker.C:
			db, connectErr = sql.Open("mysql", dsn)
			if connectErr == nil {
				if err := db.Ping(); err == nil {
					t.Log("MySQLコンテナの準備完了")
					return db
				}
				db.Close()
			}
			t.Logf("接続試行エラー: %v", connectErr)
		case <-ctx.Done():
			t.Fatalf("タイムアウト: MySQLコンテナに接続できません。最後のエラー: %v", connectErr)
		}
	}
}

// setupIntegrationTest はMySQL Dockerコンテナを起動し、テスト用DBを準備します。
func setupIntegrationTest(t *testing.T) (*sql.DB, func()) {
	if os.Getenv("SKIP_INTEGRATION") == "1" {
		t.Skip("環境変数SKIP_INTEGRATIONが設定されているため、インテグレーションテストをスキップします")
	}

	startDockerContainer(t)

	cleanup := func() {
		removeContainer(t)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&timeout=10s",
		testDBUser, testDBPassword, testDBHost, testDBPort, testDBName)
	t.Logf("接続DSN: %s", dsn)

	db := waitForMySQL(t, dsn)

	// テーブル作成
	if _, err := db.Exec(createTableSQL); err != nil {
		cleanup()
		t.Fatalf("テーブル作成エラー: %v", err)
	}

	// テスト用データ投入
	if _, err := db.Exec("INSERT INTO stocks (name, amount) VALUES (?, ?)", "apple", 100); err != nil {
		cleanup()
		t.Fatalf("テストデータ挿入エラー: %v", err)
	}

	return db, cleanup
}

// 以下はテスト関数の例です。
// 例として、実際のDB接続とクエリを検証するテストケースを記述しています。

func TestIntegrationDBConnection(t *testing.T) {
	db, cleanup := setupIntegrationTest(t)
	defer cleanup()

	t.Run("実DB接続テスト", func(t *testing.T) {
		if err := db.Ping(); err != nil {
			t.Fatalf("DB Pingエラー: %v", err)
		}
		t.Log("実DBへの接続とPing成功")
	})

	t.Run("実DBでのクエリテスト", func(t *testing.T) {
		results, err := QueryStocks(db, "apple")
		if err != nil {
			t.Fatalf("QueryStocksエラー: %v", err)
		}
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
		if err := UpsertStock(db, "banana", 50); err != nil {
			t.Fatalf("UpsertStockエラー (INSERT): %v", err)
		}
		// 既存データの更新
		if err := UpsertStock(db, "apple", 200); err != nil {
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
