// Package score 负责评分曲线、profile 加载与聚合。
package score

import (
	"embed"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
)

//go:embed profiles/*.json
var profilesFS embed.FS

// Profile 是评分参数配置（版本化，任何数字变动必须升版本）。
type Profile struct {
	Version string      `json:"version"`
	Weights []Dimension `json:"weights"`
	K       float64     `json:"k"`
	Preview bool        `json:"preview"`
}

// Dimension 单维权重与参考值。
type Dimension struct {
	Key    string    `json:"key"`
	Weight int       `json:"weight"`
	Refs   []ItemRef `json:"refs"`
}

// ItemRef 子项参考值（用于 calibrate 输出）。
type ItemRef struct {
	ID   string  `json:"id"`
	Ref  float64 `json:"ref"`
	Unit string  `json:"unit"`
}

// Load 加载指定版本 profile。version 为空时取最新。
func Load(version string) (Profile, error) {
	entries, err := profilesFS.ReadDir("profiles")
	if err != nil {
		return Profile{}, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	if len(names) == 0 {
		return Profile{}, errNoProfile
	}
	name := names[0]
	if version != "" {
		cand := "profiles/" + version + ".json"
		for _, n := range names {
			if "profiles/"+n == cand {
				name = n
				break
			}
		}
		if !strings.HasPrefix(name, version) {
			name = names[0]
		}
	}
	data, err := profilesFS.ReadFile("profiles/" + name)
	if err != nil {
		return Profile{}, err
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return Profile{}, err
	}
	return p, nil
}

var errNoProfile = &profileErr{"no profile embedded"}

type profileErr struct{ msg string }

func (e *profileErr) Error() string { return e.msg }

// LatestVersion 返回内嵌最新 profile 版本号。
func LatestVersion() string {
	p, err := Load("")
	if err != nil {
		return "0.0.0"
	}
	return p.Version
}

// DimRatio 维度内加权几何平均。
func DimRatio(items []ScoredItem) float64 {
	var s, sw float64
	for _, it := range items {
		if !it.Scored || it.Ratio <= 0 {
			continue
		}
		r := clampF(it.Ratio, 0.01, 50)
		s += it.Weight * math.Log(r)
		sw += it.Weight
	}
	if sw == 0 {
		return 0
	}
	return math.Exp(s / sw)
}

// ScoredItem 评分输入。
type ScoredItem struct {
	Ratio  float64
	Weight float64
	Scored bool
}

// DimScore 饱和曲线。
func DimScore(weight int, ratio float64, k float64) int {
	if ratio <= 0 {
		return 0
	}
	v := float64(weight) * (1 - math.Exp(-k*ratio))
	return int(v + 0.5)
}

// Normalize 某维度缺失时，把已有得分按权重归一化到 full。
func Normalize(scored []int, weights []int, full int) (int, bool) {
	total := 0
	weightSum := 0
	for i, s := range scored {
		if i < len(weights) && s > 0 {
			total += s
			weightSum += weights[i]
		}
	}
	if weightSum == 0 {
		return 0, false
	}
	return int(float64(total) * float64(full) / float64(weightSum)), true
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var _ = strconv.Itoa
