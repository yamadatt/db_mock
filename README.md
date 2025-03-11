## このリポジトリは

DBをモックにしてテストする手法がピンとこないので、実際にデータベースのモックにフォーカスして素振りするために作ったもの。

以下のテストを実施する。

## アプリケーションがやっていること

- テーブルに在庫を登録する
  - データがなければアイテムと在庫数をinsert
  - データがあれば在庫の数をupdate
- 登録した状態を確認する
  - sselect

## テスト内容

- ユニットテスト
  - DBをモックでテスト

- インテグレーションテスト
  - dockerで使い捨てのデータベースを使う

- E2Eテスト
    - 検証用データベース（ちゃんとDBとしてつかっているやつ）を使用する


## コマンド

unit-test:

```bash
SKIP_INTEGRATION=1 go test -v -race ./...
```

integration-test:

```bash
	go test -v -run "Integration" ./...
```

テストのカバレッジまで出力する。



```bash
go test -v --cover
```





func main() {
	// データベースに接続
	db, err := ConnectDB()
	if err != nil {
		log.Fatalf("DB接続に失敗しました: %v", err)
	}
	defer db.Close()
}






// ConnectDB はMySQLデータベースへの接続を確立します。
func ConnectDB() (*sql.DB, error) {
	// DSNフォーマット: user:password@tcp(host:port)/dbname?parseTime=true
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := openDBFunc("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return db, nil
}


依存注入するには

先頭にこれを入れる。

var openDBFunc = sql.Open

sql.Open は Go の標準 database/sql パッケージの関数で、指定したドライバとデータソース名（DSN）を使用してデータベース接続を確立します。

ユニットテスト：テスト時にモックdbを入れる
インテグレーションテスト：dockerのDBを入れる
E2Eテスト：検証環境のRDSを入れる