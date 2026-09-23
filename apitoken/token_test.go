package apitoken

import (
	"fmt"
	"testing"
)

type testCursor struct {
	Name    string
	Version string
}

func checkTokenEncoding(t *testing.T, tc *testCase) {
	encodedTok := EncodeJsonToken(tc.cursor)
	t.Log("Encoded token: ", tc.cursor, encodedTok)

	var decodedCursor testCursor
	isEmpty, err := DecodeJsonToken(encodedTok, &decodedCursor)
	if err != nil {
		t.Fatal("failed to decode token", err)
	}

	if isEmpty != tc.ExpectedIsEmpty() {
		t.Fatalf("isEmpty did not match expected (expected=%t, actual=%t)", tc.ExpectedIsEmpty(), isEmpty)
	}

	if !isEmpty {
		if tc.expectedOutputCursor != decodedCursor {
			t.Fatalf("cursor did not match expected (expected=%+v, actual=%+v)", tc.expectedOutputCursor, decodedCursor)
		}
	}
}

type testCase struct {
	cursor               *testCursor
	expectedOutputCursor testCursor
}

func (tc *testCase) ExpectedIsEmpty() bool {
	return tc.cursor == nil
}

func TestTokenEncoding(t *testing.T) {
	testCases := []*testCase{
		{
			cursor:               &testCursor{Name: "hi", Version: "1.0"},
			expectedOutputCursor: testCursor{Name: "hi", Version: "1.0"},
		},
		{
			cursor:               nil,
			expectedOutputCursor: testCursor{Name: "", Version: ""},
		},
	}

	for i, _ := range testCases {
		name := fmt.Sprintf("tc-%d", i)
		tc := testCases[i]
		t.Run(name, func(t *testing.T) {
			checkTokenEncoding(t, tc)
		})
	}
}
