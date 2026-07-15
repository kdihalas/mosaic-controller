package extraction

import "testing"

func FuzzSafeName(f *testing.F) {
	for _, seed := range []string{"file", "a/b", "../escape", "/absolute", `C:\\windows`, "a/../../b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, name string) {
		got, err := safeName(name, Limits{MaxPathBytes: 4096, MaxDepth: 64})
		if err == nil && (got == ".." || len(got) >= 3 && got[:3] == "../") {
			t.Fatalf("unsafe path accepted: %q", got)
		}
	})
}
