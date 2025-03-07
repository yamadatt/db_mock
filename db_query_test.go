package main

import (
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
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
		// ※ 必要に応じて、他のテストケースを追加できます
	}

	for _, tc := range tests {
		tc := tc // ループ変数の再束縛
		t.Run(tc.name, func(t *testing.T) {
			// 共通関数を使用してモックDBを設定
			db, mock, _ := setupMockDB(t)
			defer db.Close()

			// 期待するクエリと返却行の設定
			mock.ExpectQuery(tc.mockQueryRegex).
				WithArgs(tc.queryArg).
				WillReturnRows(tc.mockRows)

			// テスト対象関数の実行
			results, err := QueryStocks(db, tc.queryArg)
			if tc.expectError {
				if err == nil {
					t.Fatalf("エラー発生を期待しましたが、nilが返されました")
				}
				// エラー内容の検証を追加可能
				return
			}

			if err != nil {
				t.Fatalf("予期せぬエラー: %v", err)
			}

			// 結果の件数の検証
			if len(results) != len(tc.expectedResult) {
				t.Fatalf("期待される行数は%dでしたが、%d行返されました", len(tc.expectedResult), len(results))
			}

			// 返却結果全体をDeepEqualで検証
			for i, expected := range tc.expectedResult {
				for key, expectedVal := range expected {
					if results[i][key] != expectedVal {
						t.Errorf("結果[%d]の'%s'が期待値'%v'と異なります（実際: '%v'）", i, key, expectedVal, results[i][key])
					}
				}
			}

			// 共通関数を使用してモックの期待検証
			verifyExpectations(t, mock)
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
			if err == nil {
				t.Fatal("エラーを期待していましたが、nilが返されました")
			}
			// errors.Is を使ったエラー比較
			if !errors.Is(err, tc.expectedErr) {
				t.Fatalf("期待されるエラー '%v' と実際のエラー '%v' が一致しません", tc.expectedErr, err)
			}

			// 共通関数を使用してモックの期待検証
			verifyExpectations(t, mock)
		})
	}
}
