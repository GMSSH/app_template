// 本目录只放文案 ini。这个文件负责在编译期把它们嵌进二进制，不要加业务逻辑。
package locales

import "embed"

//go:embed zh-CN.ini en.ini
var Files embed.FS
