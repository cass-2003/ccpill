package gitinfo

import "testing"

func TestParsePorcelainV2(t *testing.T) {
	out := "# branch.oid abc123\n" +
		"# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +2 -1\n" +
		"1 .M N... 100644 100644 100644 abc def README.md\n" +
		"? untracked.txt\n"
	info := parsePorcelainV2(out)
	if !info.IsRepo || info.Branch != "main" || info.Ahead != 2 || info.Behind != 1 || info.Dirty != 2 {
		t.Errorf("parsePorcelainV2 = %+v", info)
	}
}

func TestCollectNonRepo(t *testing.T) {
	if info := Collect(t.TempDir()); info.IsRepo {
		t.Error("非 git 目录应返回 IsRepo=false")
	}
}
