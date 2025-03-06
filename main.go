package main

import (
	"fmt"
	"log"
)

func main() {
	// データベースに接続
	db, err := ConnectDB()
	if err != nil {
		log.Fatalf("DB接続に失敗しました: %v", err)
	}
	defer db.Close()

	// 接続確認
	if err := PingDB(db); err != nil {
		log.Fatalf("DB接続確認に失敗しました: %v", err)
	}

	// stocksテーブルから"name"が"apple"のレコードを取得
	results, err := QueryStocks(db, "apple")
	if err != nil {
		log.Fatalf("クエリ実行に失敗しました: %v", err)
	}

	// 取得結果の表示
	if len(results) == 0 {
		fmt.Println("結果が見つかりませんでした。")
	} else {
		fmt.Printf("全ての行: %v\n", results)
	}

	fmt.Println("クエリの実行が完了しました。")

	// 例: "apple"の在庫を200追加
	err = UpsertStock(db, "apple", 200)
	if err != nil {
		log.Fatalf("在庫更新エラー: %v", err)
	}
	fmt.Println("在庫データが更新されました")
}
