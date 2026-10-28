---
name: go-testing
description: Write or fix Go tests - table-driven tests, subtests, t.TempDir, httptest fakes, running go test
---

# Writing Go tests

1. Find the existing style first: `go test ./... 2>&1 | head`, and read one existing `_test.go` in the same package.
2. Prefer table-driven tests with `t.Run(tc.name, ...)` so each case is reported separately.
3. Use `t.TempDir()` for files and `httptest.NewServer` for HTTP; never touch the real network or home directory.
4. Compare with plain `if got != want { t.Fatalf("got %v, want %v", got, want) }` unless the repo already uses testify.
5. Test behavior through exported functions; do not export things just to test them.
6. After writing tests, run `go test ./... -race -count=1` and `go vet ./...`, and fix every failure before reporting done.

Skeleton:

```go
func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int
		wantErr bool
	}{
		{"simple", "42", 42, false},
		{"empty", "", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
```
