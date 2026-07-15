package gitinfo

import "testing"

func TestParsePorcelainV2(t *testing.T) {
	out := "# branch.oid abc123\n" +
		"# branch.head main\n" +
		"# branch.upstream origin/main\n" +
		"# branch.ab +2 -1\n" +
		"# stash 3\n" +
		"1 .M N... 100644 100644 100644 abc def README.md\n" +
		"1 M. N... 100644 100644 100644 abc def staged.go\n" +
		"1 MM N... 100644 100644 100644 abc def both.go\n" +
		"u UU N... 100644 100644 100644 100644 abc def ghi conflict.go\n" +
		"? untracked.txt\n"
	info := parsePorcelainV2(out)
	if !info.IsRepo || info.Branch != "main" || info.Ahead != 2 || info.Behind != 1 || info.Dirty != 5 {
		t.Errorf("parsePorcelainV2 基础字段 = %+v", info)
	}
	if info.Upstream != "origin/main" || info.Stash != 3 {
		t.Errorf("upstream/stash = %+v", info)
	}
	if info.Staged != 2 || info.Unstaged != 2 || info.Untracked != 1 || info.Conflicts != 1 {
		t.Errorf("分类计数 = %+v", info)
	}
}

func TestParseShortStat(t *testing.T) {
	d := parseShortStat(" 3 files changed, 42 insertions(+), 10 deletions(-)")
	if !d.OK || d.Ins != 42 || d.Del != 10 {
		t.Errorf("parseShortStat = %+v", d)
	}
	if d := parseShortStat(""); !d.OK || d.Ins != 0 || d.Del != 0 {
		t.Errorf("空输出应为 0/0: %+v", d)
	}
	if d := parseShortStat(" 1 file changed, 1 insertion(+)"); d.Ins != 1 || d.Del != 0 {
		t.Errorf("单数形式: %+v", d)
	}
}

func TestParseRemoteURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/anthropics/claude-code.git": "anthropics/claude-code",
		"git@github.com:owner/repo.git":                 "owner/repo",
		"ssh://git@github.com/owner/repo":               "owner/repo",
		"https://gitlab.com/group/sub/repo.git":         "sub/repo",
		"":                                              "",
		"https://github.com":                            "",
	}
	for in, want := range cases {
		if got := ParseRemoteURL(in); got != want {
			t.Errorf("ParseRemoteURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCollectNonRepo(t *testing.T) {
	if info := Collect(t.TempDir()); info.IsRepo {
		t.Error("非 git 目录应返回 IsRepo=false")
	}
}
