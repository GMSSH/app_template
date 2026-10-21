package score

import (
	"os"
	"path/filepath"
	"testing"

	"gmssh-cli-plugin/internal/bench"
	"gmssh-cli-plugin/internal/input"
	"gmssh-cli-plugin/internal/middleware"
	"gmssh-cli-plugin/internal/store"
)

func TestScoreBounds(t *testing.T) {
	ctx := &middleware.Context{Store: store.New(t.TempDir())}
	raw, err := RunScore(ctx, input.Input{})
	if err != nil {
		t.Fatal(err)
	}
	got := raw.(Result)
	if got.CPU < 0 || got.CPU > CPUWeight || got.Mem < 0 || got.Mem > MemWeight || got.Disk < 0 || got.Disk > DiskWeight {
		t.Fatalf("out of range: %+v", got)
	}
	if got.Total != got.CPU+got.Mem+got.Disk || len(got.Parts) != 3 {
		t.Fatalf("result %+v", got)
	}
}

func TestHostFields(t *testing.T) {
	host := ReadHost()
	if host.Cores <= 0 || host.CPU == "" {
		t.Fatalf("host %+v", host)
	}
}

func TestOverviewUsesSavedScore(t *testing.T) {
	ctx := &middleware.Context{Store: store.New(t.TempDir())}
	raw, err := RunScore(ctx, input.Input{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := RunOverview(ctx, input.Input{})
	if err != nil {
		t.Fatal(err)
	}
	overview := out.(Overview)
	if overview.Host.Cores <= 0 || overview.Latest == nil || overview.Latest.Total != raw.(Result).Total || len(overview.Trend) != 1 {
		t.Fatalf("overview %+v", overview)
	}
}

func probeArgs() []input.Arg {
	return []input.Arg{
		{Name: "key", Type: input.TypeString},
		{Name: "dir", Type: input.TypeString},
	}
}

func probeInput(key, dir string) input.Input {
	argv := []string{"--key", key}
	if dir != "" {
		argv = append(argv, "--dir", dir)
	}
	in, _ := input.Parse("probe", probeArgs(), argv)
	return in
}

func TestProbeRejectsUnknown(t *testing.T) {
	in, err := input.Parse("probe", probeArgs(), []string{"--key", "network"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := RunProbe(&middleware.Context{}, in); err == nil {
		t.Fatal("network should be rejected")
	}
}

func TestRunSavesHistory(t *testing.T) {
	dir := t.TempDir()
	ctx := &middleware.Context{Store: store.New(dir)}
	raw, err := RunScore(ctx, input.Input{})
	if err != nil {
		t.Fatal(err)
	}
	result := raw.(Result)
	var points []Point
	if err := ctx.Store.ReadJSON("scores.json", &points); err != nil {
		t.Fatal(err)
	}
	if len(points) != 1 || points[0].Score != result.Total {
		t.Fatalf("history %+v", points)
	}
	if _, err := os.Stat(filepath.Join(dir, "scores.json")); err != nil {
		t.Fatal(err)
	}
}

func TestProbeOutputContract(t *testing.T) {
	// 前端契约字段必须齐全：key/score/weight/value/unit/percent
	raw, err := RunProbe(&middleware.Context{}, probeInput("cpu", ""))
	if err != nil {
		t.Fatal(err)
	}
	r := raw.(bench.Result)
	if r.Key != "cpu" || r.Weight != 4000 || r.Max != 4000 {
		t.Fatalf("contract %+v", r)
	}
	if r.Status != "ok" && r.Status != "refused" && r.Status != "busy" && r.Status != "aborted" && r.Status != "partial" && r.Status != "unsupported" {
		t.Fatalf("bad status %s", r.Status)
	}
	if r.Score > 4000 {
		t.Fatalf("score overflow %d", r.Score)
	}
}
