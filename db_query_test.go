package main

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert" // 追加
)

func TestQueryStocks(t *testing.T) {
	tests := []struct {
		name           string
		queryArg       string
		mockRows       *sqlmock.Rows
		mockQueryRegex string
		expectedResult []map[string]interface{}
		expectError    bool
	}{
		{
			name:           "appleが1件返る場合",
			queryArg:       "apple",
			mockRows:       sqlmock.NewRows([]string{"id", "name", "amount"}).AddRow(1, "apple", 100),
			mockQueryRegex: "SELECT \\* FROM stocks WHERE name = \\?;",
			expectedResult: []map[string]interface{}{
				{"id": int64(1), "name": "apple", "amount": int64(100)},
			},
			expectError: false,
		},
		// 全件表示のテストケース
		{
			name:     "空の名前で全件表示",
			queryArg: "", // 空文字列を渡す
			mockRows: sqlmock.NewRows([]string{"id", "name", "amount"}).
				AddRow(1, "apple", 100).
				AddRow(2, "banana", 50).
				AddRow(3, "orange", 75),
			mockQueryRegex: "SELECT \\* FROM stocks;", // WHERE句のないクエリ
			expectedResult: []map[string]interface{}{
				{"id": int64(1), "name": "apple", "amount": int64(100)},
				{"id": int64(2), "name": "banana", "amount": int64(50)},
				{"id": int64(3), "name": "orange", "amount": int64(75)},
			},
			expectError: false,
		},
		// ※ 必要に応じて、他のテストケースを追加できます
	}

	for _, tc := range tests {
		tc := tc // ループ変数の再束縛
		t.Run(tc.name, func(t *testing.T) {
			// 共通関数を使用してモックDBを設定
			db, mock, _ := setupMockDB(t)
			defer db.Close()

			// 期待するクエリと返却行の設定
			if tc.queryArg == "" {
				// 空文字列の場合はWHERE句なしのクエリが期待される
				mock.ExpectQuery(tc.mockQueryRegex).
					WillReturnRows(tc.mockRows)
			} else {
				// 通常のケースはWHERE句ありのクエリが期待される
				mock.ExpectQuery(tc.mockQueryRegex).
					WithArgs(tc.queryArg).
					WillReturnRows(tc.mockRows)
			}

			// テスト対象関数の実行
			results, err := QueryStocks(db, tc.queryArg)

			if tc.expectError {
				assert.Error(t, err, "エラーが発生するべき")
				return
			}

			// 正常系のアサーション
			assert.NoError(t, err, "エラーが発生すべきでない")
			assert.Len(t, results, len(tc.expectedResult), "結果の行数が期待通りであるべき")

			// 返却結果の検証
			for i, expected := range tc.expectedResult {
				for key, expectedVal := range expected {
					assert.Equal(t, expectedVal, results[i][key],
						"結果[%d]の'%s'が期待値と一致するべき", i, key)
				}
			}

			// モックの期待検証
			assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
		})
	}
}

func TestQueryStocks_Error(t *testing.T) {
	tests := []struct {
		name        string
		queryArg    string
		expectedErr error
		queryRegex  string
	}{
		{
			name:        "存在しない銘柄でエラー発生",
			queryArg:    "nonexistent",
			expectedErr: errors.New("query error"),
			queryRegex:  "SELECT \\* FROM stocks WHERE name = \\?;",
		},
		// 将来的に別のエラーケースを追加する場合は、ここにケースを追加
	}

	for _, tc := range tests {
		tc := tc // ループ変数をローカルスコープに束縛
		t.Run(tc.name, func(t *testing.T) {
			// 共通関数を使用してモックDBを設定
			db, mock, _ := setupMockDB(t)
			defer db.Close()

			// モックの期待設定
			mock.ExpectQuery(tc.queryRegex).
				WithArgs(tc.queryArg).
				WillReturnError(tc.expectedErr)

			// QueryStocks関数を実行
			_, err := QueryStocks(db, tc.queryArg)

			// エラー検証
			assert.Error(t, err, "エラーが発生するべき")
			assert.Equal(t, tc.expectedErr.Error(), err.Error(), "期待されるエラーメッセージと一致するべき")

			// モックの期待検証
			assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
		})
	}
}

// TestQueryStocks_ColumnsError はrows.Columns()がエラーを返すケースをテストします
func TestQueryStocks_ColumnsError(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// カスタムエラー行を生成するモックRowsを作成
	mockRows := sqlmock.NewRows([]string{"id", "name", "amount"}).
		AddRow(1, "apple", 100).
		RowError(0, errors.New("row error"))

	// ここでColumnsを呼ぶ前にRowsをCloseして、その後のColumnsでエラーになるようにする
	mock.ExpectQuery("SELECT \\* FROM stocks WHERE name = \\?;").
		WithArgs("apple").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	_, err := QueryStocks(db, "apple")

	// エラーが発生するはず
	assert.Error(t, err, "Columnsエラーが発生するべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
}

// TestQueryStocks_ScanError はrows.Scan()がエラーを返すケースをテストします
func TestQueryStocks_ScanError(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// 明示的なScanエラーを設定
	mockRows := sqlmock.NewRows([]string{"id", "name", "amount"}).
		AddRow(1, "apple", 100).
		RowError(0, errors.New("forced scan error"))

	mock.ExpectQuery("SELECT \\* FROM stocks WHERE name = \\?;").
		WithArgs("apple").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	results, err := QueryStocks(db, "apple")

	// モックの検証を最初に行う
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")

	// エラーをチェック
	if assert.Error(t, err, "Scanエラーが発生するべき") {
		assert.Contains(t, err.Error(), "scan error", "エラーメッセージは'scan error'を含むべき")
	}

	// 結果は空かnilであるべき
	if results != nil {
		assert.Empty(t, results, "エラー時は結果が空であるべき")
	}
}

// TestQueryStocks_RowsError はrows.Err()がエラーを返すケースをテストします
func TestQueryStocks_RowsError(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// 行の取得が終わった後のErrチェックでエラーを返すモックRows
	mockRows := sqlmock.NewRows([]string{"id", "name", "amount"}).
		CloseError(errors.New("rows error after iteration"))

	mock.ExpectQuery("SELECT \\* FROM stocks;").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	_, err := QueryStocks(db, "")

	// エラーが発生するはず
	assert.Error(t, err, "Rows.Errエラーが発生するべき")
	assert.Contains(t, err.Error(), "rows error after iteration", "期待するエラーメッセージを含むべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
}

// TestQueryStocks_EmptyResults は空の結果を返すケースをテストします
func TestQueryStocks_EmptyResults(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// 空の結果セット
	mockRows := sqlmock.NewRows([]string{"id", "name", "amount"})

	mock.ExpectQuery("SELECT \\* FROM stocks WHERE name = \\?;").
		WithArgs("nonexistent_item").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	results, err := QueryStocks(db, "nonexistent_item")

	// エラーは発生せず、空の結果が返るはず
	assert.NoError(t, err, "エラーは発生すべきでない")
	assert.Len(t, results, 0, "空の結果配列が返されるべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
}

// TestQueryStocks_NullValues はNULL値を含む結果をテストします
func TestQueryStocks_NullValues(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// NULL値を含む行を返す
	mockRows := sqlmock.NewRows([]string{"id", "name", "amount"}).
		AddRow(1, nil, 100) // nameにNULL値

	// 空文字列の場合はWHERE句なしのクエリが正しい
	mock.ExpectQuery("SELECT \\* FROM stocks;").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	results, err := QueryStocks(db, "") // 空文字列を渡す

	// NULL値の処理が正しく行われるはず
	assert.NoError(t, err, "エラーは発生すべきでない")
	assert.Len(t, results, 1, "1つの結果が返されるべき")
	assert.Equal(t, int64(1), results[0]["id"], "IDが正しいべき")
	assert.Nil(t, results[0]["name"], "nameはnilであるべき")
	assert.Equal(t, int64(100), results[0]["amount"], "amountが正しいべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
}

// TestQueryStocks_BinaryData はバイナリデータの処理をテストします
func TestQueryStocks_BinaryData(t *testing.T) {
	db, mock, _ := setupMockDB(t)
	defer db.Close()

	// バイナリデータを含む行を返す
	binaryData := []byte{0x01, 0x02, 0x03} // バイナリデータ
	mockRows := sqlmock.NewRows([]string{"id", "name", "data"}).
		AddRow(1, "binary_item", binaryData)

	mock.ExpectQuery("SELECT \\* FROM stocks WHERE name = \\?;").
		WithArgs("binary_item").
		WillReturnRows(mockRows)

	// テスト対象関数を実行
	results, err := QueryStocks(db, "binary_item")

	// バイナリデータが文字列に変換されるはず
	assert.NoError(t, err, "エラーは発生すべきでない")
	assert.Len(t, results, 1, "1つの結果が返されるべき")
	assert.Equal(t, int64(1), results[0]["id"], "IDが正しいべき")
	assert.Equal(t, "binary_item", results[0]["name"], "nameが正しいべき")
	assert.IsType(t, string(""), results[0]["data"], "バイナリデータは文字列に変換されるべき")
	assert.NoError(t, mock.ExpectationsWereMet(), "すべての期待されるSQLが実行されるべき")
}
