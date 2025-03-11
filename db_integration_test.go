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
	"github.com/stretchr/testify/assert" // 追加
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

	startTime := time.Now()
	t.Logf("MySQLコンテナに接続を試行中... (開始時刻: %v)", startTime.Format("15:04:05"))
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
			elapsed := time.Since(startTime)
			t.Logf("接続試行エラー: %v (経過時間: %v) 起動まで約1分45秒ぐらいかかる", connectErr, elapsed)
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
		// assertを使って簡潔に記述
		assert.NoError(t, db.Ping(), "DBへの接続とPingは成功すべき")
		t.Log("実DBへの接続とPing成功")
	})

	t.Run("実DBでのクエリテスト", func(t *testing.T) {
		results, err := QueryStocks(db, "apple")

		// エラーチェック
		assert.NoError(t, err, "QueryStocksは成功すべき")

		// 結果の検証
		if assert.Len(t, results, 1, "結果は1件のみ存在すべき") {
			// 結果が存在する場合のみフィールド検証
			assert.Equal(t, "apple", results[0]["name"], "結果の商品名が一致すべき")
			assert.Equal(t, int64(100), results[0]["amount"], "結果の数量が一致すべき")
		}

		t.Log("実DBでのクエリテスト成功")
	})

	t.Run("実DBでのUpsertテスト", func(t *testing.T) {
		fmt.Println("=== 実DBでのUpsertテスト 開始 ===")

		// 新規データのUpsert
		fmt.Println("新規データ 'banana' をUpsert (数量: 50)")
		assert.NoError(t, UpsertStock(db, "banana", 50), "新規データのUpsertは成功すべき")

		// 既存データの更新
		fmt.Println("既存データ 'apple' をUpsert (数量: 200、初期値は100)")
		assert.NoError(t, UpsertStock(db, "apple", 200), "既存データの更新は成功すべき")

		// 変更を確認
		fmt.Println("更新後のデータを確認中...")
		results, err := QueryStocks(db, "apple")
		assert.NoError(t, err, "更新後のQueryStocksは成功すべき")

		if assert.Len(t, results, 1, "更新後も結果は1件のみ存在すべき") {
			fmt.Printf("確認結果: apple の数量 = %d (期待値: 300)\n", results[0]["amount"])
			assert.Equal(t, int64(300), results[0]["amount"], "更新後の数量は300になるべき")
		}

		//全件表示して、appleが300、bananaが50になっていることを確認
		fmt.Println("全データを表示中...")
		results, err = QueryStocks(db, "")
		assert.NoError(t, err, "全データのQueryStocksは成功すべき")

		if assert.Len(t, results, 2, "全データは2件存在すべき") {

			for _, r := range results {
				fmt.Printf("商品名: %s, 数量: %d\n", r["name"], r["amount"])
				assert.Equal(t, int64(300), results[0]["amount"], "appleの数量は300になるべき")
				assert.Equal(t, int64(50), results[1]["amount"], "bananaの数量は50になるべき")
			}

			fmt.Println("=== 実DBでのUpsertテスト 完了 ===")
			t.Log("実DBでのUpsertテスト成功")
		}
	})
}
