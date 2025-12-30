package reasons

import "strings"

var reasonByLang = map[string]string{
	"go":     "编译快、部署简单、并发模型清晰，适合做基础设施和服务端。",
	"python": "生态丰富、验证快，适合数据处理和脚本。",
	"java":   "工程成熟、生态庞大，适合大型企业级系统。",
}

// Reason returns a short reason for a language.
func Reason(lang string) string {
	key := strings.ToLower(strings.TrimSpace(lang))
	if key == "" {
		key = "go"
	}
	if reason, ok := reasonByLang[key]; ok {
		return reason
	}
	return "先选一个目标场景，再决定语言。Go 适合服务端与工具链。"
}
