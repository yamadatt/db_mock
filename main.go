package main

import (
	"database/sql"
	"fmt"
	"log"
)

// mainProcessは、商品名と数量を受け取って処理を行います。
// main()からの呼び出し時にはハードコードした値を渡し、
// テスト時には任意の値をモックできるようになります。
func mainProcess(db *sql.DB, productName string, amount int) error {
	// 接続確認
	if err := PingDB(db); err != nil {
		return fmt.Errorf("DB接続確認に失敗しました: %v", err)
	}

	// stocksテーブルから"name"が"apple"のレコードを取得
	results, err := QueryStocks(db, productName)
	if err != nil {
		return fmt.Errorf("クエリ実行に失敗しました: %v", err)
	}

	// 取得結果の表示
	if len(results) == 0 {
		fmt.Println("結果が見つかりませんでした。")
	} else {
		fmt.Printf("全ての行: %v\n", results)
	}

	fmt.Println("クエリの実行が完了しました。")

	// 例: "apple"の在庫を200追加
	err = UpsertStock(db, productName, amount)
	if err != nil {
		return fmt.Errorf("在庫更新エラー: %v", err)
	}
	fmt.Println("在庫データが更新されました")
	return nil
}

func main() {
	// 固定値はここで定義
	productName := "apple"
	amount := 200

	db, err := ConnectDB()
	if err != nil {
		log.Fatalf("DB接続に失敗しました: %v", err)
	}
	defer db.Close()

	// 処理を委譲
	err = mainProcess(db, productName, amount)
	if err != nil {
		log.Fatalf("処理に失敗しました: %v", err)
	}
}
