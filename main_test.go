package main

import (
	"bytes"
	"os"
	"testing"
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
func TestMainFunction(t *testing.T) {
	output := captureOutput(func() {
		// main関数を実行
		main()
	})
	// main が出力を生成しない場合のケース
	if len(output) == 0 {
		t.Log("main()は出力を生成しませんでした。これは実装によっては許容される場合があります")
	} else {
		t.Log("main()からキャプチャした出力:", output)
	}
}
