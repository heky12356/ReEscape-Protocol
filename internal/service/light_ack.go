package service

import (
	"hash/fnv"
	"strings"

	"project-yume/internal/config"
)

var (
	lightAckListeningPool = []string{
		"嗯嗯",
		"然后呢然后呢",
	}
	lightAckComfortPool = []string{
		"辛苦辛苦",
		"抱抱你",
		"唉，确实难受吧",
	}
	lightAckEncouragePool = []string{
		"嗯嗯，可以的",
		"有道理",
		"是这样的",
		"慢慢来，没关系的",
		"没事哒没事哒",
	}
	lightAckDirectPool = []string{
		"知道了",
		"收到",
		"嗯嗯",
		"明白",
	}
	lightAckContinuePool = []string{
		"嗯嗯",
		"也是啊",
		"确实啊",
	}
	lightAckClosePool = []string{
		"好好，晚安。",
		"嗯嗯，拜拜。",
		"好噢",
		"彳亍",
	}
)

// SelectLightAck 为轻回应选择一句更像“接话”的短回复。
func SelectLightAck(sessionID, message string, analysis MessageAnalysis) string {
	if config.GetConfig().LightAckMode == "llm" {
		if reply := strings.TrimSpace(analysis.VisibleReply); reply != "" {
			return reply
		}
	}

	pool := selectLightAckPool(message, analysis)
	if len(pool) == 0 {
		return "嗯嗯"
	}

	index := stableLightAckIndex(sessionID, message, analysis, len(pool))
	return pool[index]
}

func selectLightAckPool(message string, analysis MessageAnalysis) []string {
	if analysis.WannaBye == "想结束对话" || analysis.SupportStrategy == "close_conversation" {
		return lightAckClosePool
	}

	if analysis.TurnStatus == "user_holds_floor" && (looksLikeContinuation(message) || needsListening(analysis)) {
		switch analysis.SupportStrategy {
		case "comfort":
			return []string{"唉，辛苦了", "抱抱你", "嗯嗯"}
		case "encourage":
			return []string{"可以的可以的", "有道理", "嗯嗯"}
		default:
			return lightAckListeningPool
		}
	}

	switch analysis.SupportStrategy {
	case "acknowledge_and_wait":
		return lightAckListeningPool
	case "comfort":
		return lightAckComfortPool
	case "encourage":
		return lightAckEncouragePool
	case "answer_directly":
		return lightAckDirectPool
	case "continue_chat":
		if analysis.TurnStatus == "user_holds_floor" {
			return lightAckListeningPool
		}
		return lightAckContinuePool
	default:
		return lightAckContinuePool
	}
}

func needsListening(analysis MessageAnalysis) bool {
	userNeed := strings.TrimSpace(analysis.UserNeed)
	if userNeed == "" {
		return false
	}

	keywords := []string{"被倾听", "继续表达", "倾诉", "补充", "说完", "接住"}
	for _, keyword := range keywords {
		if strings.Contains(userNeed, keyword) {
			return true
		}
	}
	return false
}

func looksLikeContinuation(message string) bool {
	trimmed := strings.TrimSpace(message)
	if trimmed == "" {
		return false
	}

	prefixes := []string{"然后", "还有", "而且", "就是", "不过", "以及", "另外", "其实"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}

	suffixes := []string{"...", "。。。", "……", "，", ",", "、", "然后", "还有"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(trimmed, suffix) {
			return true
		}
	}

	return false
}

func stableLightAckIndex(sessionID, message string, analysis MessageAnalysis, size int) int {
	if size <= 1 {
		return 0
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.TrimSpace(sessionID)))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(strings.TrimSpace(message)))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(analysis.SupportStrategy))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(analysis.TurnStatus))
	_, _ = hasher.Write([]byte("|"))
	_, _ = hasher.Write([]byte(analysis.UserNeed))

	return int(hasher.Sum32() % uint32(size))
}
