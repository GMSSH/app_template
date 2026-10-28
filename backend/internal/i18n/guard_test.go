package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// 本文件是文案（i18n）的回归护栏。背景：
//   - T() 在找不到 key 时会原样返回 key，缺 key 会把 "cmd.xxx.summary" 这类
//     内部标识直接暴露给用户；
//   - zh-CN 与 en 必须成对维护（key 集合、占位符序列一致），否则切语言后
//     会出现「中文有英文没有」或「中英占位符顺序颠倒」的缺陷。

// messageKeyPattern 匹配源码中出现的文案 key 字面量：cli.* / help.* / group.* /
// cmd.* / arg.*。环境变量名（GMCLI_*）不是文案 key，另行排除。
var messageKeyPattern = regexp.MustCompile(`"(cli\.[A-Za-z0-9_.]+|help\.[A-Za-z0-9_.]+|group\.[A-Za-z0-9_.]+|cmd\.[A-Za-z0-9_.]+|arg\.[A-Za-z0-9_.]+)"`)

// envNameAllowlist 环境变量名不是文案 key
var envNameAllowlist = map[string]bool{
	"GMCLI_LANG":     true,
	"GMCLI_I18N_DIR": true,
}

// placeholderPattern 匹配模板占位符 {}
var placeholderPattern = regexp.MustCompile(`\{\}`)

// placeholderSequence 返回占位符出现次数序列，用于发现「个数/顺序不一致」。
func placeholderSequence(value string) []string {
	matches := placeholderPattern.FindAllString(value, -1)
	seq := make([]string, 0, len(matches))
	for range matches {
		seq = append(seq, "value")
	}
	return seq
}

// moduleRoot returns the module root (tests may run under internal/*).
func moduleRoot() string { return filepath.Join("..", "..") }

// referencedKeys 扫描模块内所有非测试 Go 源码，收集被引用的文案 key
func referencedKeys(t *testing.T) map[string]bool {
	t.Helper()
	root := moduleRoot()
	keys := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path == root {
				return nil
			}
			switch entry.Name() {
			case "build", "main.dist", "node_modules", "bin", "data":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, match := range messageKeyPattern.FindAllStringSubmatch(string(content), -1) {
			if name := match[1]; !envNameAllowlist[name] {
				keys[name] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("遍历源码失败: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("没有扫描到任何文案 key，检查正则或目录")
	}
	return keys
}

// parseMessages 读取一份 ini 文案，返回 key → 文案
func parseMessages(t *testing.T, lang string) map[string]string {
	t.Helper()
	path := filepath.Join(moduleRoot(), "i18n", lang+".ini")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	out := map[string]string{}
	for _, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(raw, "\ufeff"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}
		out[strings.TrimSpace(line[:idx])] = strings.TrimSpace(line[idx+1:])
	}
	return out
}

// TestI18nKeysExistInBothLanguages 源码引用的每个 key 都必须在两种语言里存在
func TestI18nKeysExistInBothLanguages(t *testing.T) {
	keys := referencedKeys(t)
	for _, lang := range []string{"zh-CN", "en"} {
		messages := parseMessages(t, lang)
		for key := range keys {
			if _, ok := messages[key]; !ok {
				t.Errorf("%s.ini 缺少源码引用的文案 key: %s", lang, key)
			}
		}
	}
}

// TestI18nLanguageFilesInSync 两种语言的 key 集合必须完全一致
func TestI18nLanguageFilesInSync(t *testing.T) {
	zh := parseMessages(t, "zh-CN")
	en := parseMessages(t, "en")
	for key := range zh {
		if _, ok := en[key]; !ok {
			t.Errorf("en.ini 缺少 key: %s", key)
		}
	}
	for key := range en {
		if _, ok := zh[key]; !ok {
			t.Errorf("zh-CN.ini 缺少 key: %s", key)
		}
	}
	for key, zhValue := range zh {
		enValue, ok := en[key]
		if !ok {
			continue
		}
		if zhSeq, enSeq := placeholderSequence(zhValue), placeholderSequence(enValue); !slices.Equal(zhSeq, enSeq) {
			t.Errorf("key %s 的双语占位符数量不一致: zh=%q %v, en=%q %v", key, zhValue, zhSeq, enValue, enSeq)
		}
	}
}

// TestI18nNoUnescapedQuote 文案值里不能出现未转义的双引号（解析器不处理转义）
func TestI18nNoUnescapedQuote(t *testing.T) {
	for _, lang := range []string{"zh-CN", "en"} {
		path := filepath.Join(moduleRoot(), "i18n", lang+".ini")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", path, err)
		}
		for lineno, raw := range strings.Split(string(content), "\n") {
			line := strings.TrimSpace(raw)
			if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
				continue
			}
			idx := strings.Index(line, "=")
			if idx <= 0 {
				continue
			}
			value := strings.TrimSpace(line[idx+1:])
			if !strings.HasPrefix(value, `"`) {
				continue
			}
			// 去掉首尾引号后，正文里不应再有双引号
			body := strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
			if strings.Contains(body, `"`) {
				t.Errorf("%s.ini 第 %d 行含未转义双引号: %s", lang, lineno+1, line)
			}
		}
	}
}

// TestI18nPlaceholderSequenceGuard 护栏自检：序列比对必须能区分数量差异，
// 否则护栏本身可能是「永真」的
func TestI18nPlaceholderSequenceGuard(t *testing.T) {
	cases := []struct {
		value string
		want  []string
	}{
		{"未知命令: {}", []string{"value"}},
		{"参数 --{} 的取值 {} 不被支持，可选值: {}", []string{"value", "value", "value"}},
		{"没有任何占位符", nil},
	}
	for _, tc := range cases {
		if got := placeholderSequence(tc.value); !slices.Equal(got, tc.want) {
			t.Fatalf("placeholderSequence(%q) = %v, 期望 %v", tc.value, got, tc.want)
		}
	}
	if slices.Equal(placeholderSequence("{} {}"), placeholderSequence("{}")) {
		t.Fatal("占位符数量不同应判为不一致")
	}
}

// TestTTranslatesAndFallsBack T() 命中时翻译、缺失时原样返回 key
