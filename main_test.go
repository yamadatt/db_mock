package main

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

// captureOutput は標準出力をリダイレクトして、関数実行中の出力をキャプチャします。
func captureOutput(f func()) string {
	// 実際の標準出力をバックアップ
	oldStdout := os.Stdout

	// os.Pipe() のエラーチェック
	r, w, err := os.Pipe()
	if err != nil {
		panic("os.Pipe() エラー: " + err.Error())
	}

	// 標準出力を w に切り替え
	os.Stdout = w
	// defer で必ず標準出力を復元する
	defer func() {
		os.Stdout = oldStdout
	}()

	// 関数 f() を実行
	f()

	// 書き込み側を閉じる
	w.Close()

	// パイプから出力を読み取る
	var buf bytes.Buffer
	_, err = buf.ReadFrom(r)
	if err != nil {
		panic("buf.ReadFrom() エラー: " + err.Error())
	}
	// 読み取り用のパイプもクローズする
	r.Close()

	return buf.String()
}

// TestMainFunction は main() がクラッシュせずに実行され、
// 予期される出力（もしあれば）を生成することを検証します。必要に応じて期待される出力を調整する。
// func TestMainFunction(t *testing.T) {
// 	output := captureOutput(func() {
// 		// main関数を実行
// 		main()
// 	})
// 	// main が出力を生成しない場合のケース
// 	if len(output) == 0 {
// 		t.Log("main()は出力を生成しませんでした。これは実装によっては許容される場合があります")
// 	} else {
// 		t.Log("main()からキャプチャした出力:", output)
// 	}
// }

// TestMainFunctionWithMock はモックDBを使ってmain()関数の動作をテストします
func TestMainFunctionWithMock(t *testing.T) {
	// モックDBをセットアップ
	db, mock, err := setupMockDB(t)
	if err != nil {
		t.Fatalf("モックDBのセットアップに失敗: %v", err)
	}
	defer db.Close()

	// Pingの成功を期待
	mock.ExpectPing()

	// "apple"の検索クエリとその結果をモック
	mock.ExpectQuery(`SELECT \* FROM stocks WHERE name = \?;`).
		WithArgs("apple").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "amount"}).
			AddRow(1, "apple", 100))

	// UpsertStockのモック設定（既存の"apple"に対するUPDATE）
	// 1. 既存データの確認
	mock.ExpectQuery(`SELECT amount FROM stocks WHERE name = \?`).
		WithArgs("apple").
		WillReturnRows(sqlmock.NewRows([]string{"amount"}).AddRow(100))

	// 2. トランザクションの開始
	mock.ExpectBegin()

	// 3. UPDATEクエリ (100 + 200 = 300)
	mock.ExpectExec(`UPDATE stocks SET amount = \? WHERE name = \?;`).
		WithArgs(300, "apple").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// 4. トランザクションのコミット
	mock.ExpectCommit()

	// main関数の実行と出力のキャプチャ
	output := captureOutput(func() {
		main()
	})

	// 期待された全てのクエリが実行されたか確認
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("期待されたクエリが実行されませんでした: %v", err)
	}

	// 出力内容の検証
	expectedOutputs := []string{
		"全ての行:", "apple", "100", // クエリ結果の出力
		"在庫データが更新されました", // 更新成功メッセージ
	}

	for _, expected := range expectedOutputs {
		if !strings.Contains(output, expected) {
			t.Errorf("出力に '%s' が含まれていません。実際の出力: %s", expected, output)
		}
	}

	t.Logf("main()のモックテストが成功しました。出力: %s", output)
}
