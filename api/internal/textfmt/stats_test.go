package textfmt

import "testing"

func TestFormatStats(t *testing.T) {
	in := StatsInput{
		Package: "httpx", Period: "4w",
		DateFrom: "2026-06-30", DateTo: "2026-07-27",
		Overall:        []StatPoint{{Label: "Jun 30", Downloads: 12500000}, {Label: "Jul 07", Downloads: 13100000}},
		PythonVersions: []StatPoint{{Label: "3.12", Downloads: 9000000}},
		Systems:        []StatPoint{{Label: "Linux", Downloads: 20000000}},
	}
	got := FormatStats(in)
	want := `package: httpx
period: 4w
date_range: 2026-06-30 → 2026-07-27

## weekly downloads
Jun 30	12500000
Jul 07	13100000

## python versions
3.12	9000000

## systems
Linux	20000000
`
	if got != want {
		t.Errorf("FormatStats mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatStatsEmpty(t *testing.T) {
	got := FormatStats(StatsInput{Package: "ghost", Period: "4w"})
	if got != "package: ghost\nperiod: 4w\n\n# no download data\n" {
		t.Errorf("empty stats output:\n%s", got)
	}
}
