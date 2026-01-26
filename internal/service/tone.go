package service

import "strings"

// AdjustResponseByPattern 根据对话模式调整回复
func AdjustResponseByPattern(response, pattern, emotion string) string {
	switch pattern {
	case "需要关怀":
		return AddComfortTone(response, emotion)
	case "积极活跃":
		return AddCheerfulTone(response, emotion)
	case "情绪波动":
		return AddCalmTone(response, emotion)
	case "新用户":
		return AddWelcomeTone(response, emotion)
	default:
		return response
	}
}

// AddComfortTone 添加关怀语调
func AddComfortTone(response, emotion string) string {
	comfortPrefixes := []string{"", "别担心，", "我理解你，", "没关系的，"}
	comfortSuffixes := []string{"", "，我会陪着你", "，一切都会好起来的", "，你不是一个人"}

	if emotion == "难过" {
		return "我一直都在这里陪着你" + comfortSuffixes[1]
	}
	if emotion == "生气" {
		return comfortPrefixes[1] + response + comfortSuffixes[0]
	}

	return comfortPrefixes[0] + response + comfortSuffixes[0]
}

// AddCheerfulTone 添加活泼语调
func AddCheerfulTone(response, emotion string) string {
	cheerfulPrefixes := []string{"", "哈哈，", "嘿嘿，", "哇，"}
	cheerfulSuffixes := []string{"", "！", "~", "呢！"}

	if emotion == "开心" {
		return cheerfulPrefixes[1] + "你的好心情也感染到我了" + cheerfulSuffixes[1]
	}
	if emotion == "中性" {
		return cheerfulPrefixes[0] + response + cheerfulSuffixes[1]
	}

	return response + cheerfulSuffixes[0]
}

// AddCalmTone 添加平静语调
func AddCalmTone(response, emotion string) string {
	calmPrefixes := []string{"", "嗯，", "好的，", "我明白，"}
	calmSuffixes := []string{"", "，我们慢慢聊", "，不着急", "，慢慢来"}

	if strings.Contains(response, "?") || strings.Contains(response, "？") {
		return calmPrefixes[3] + response + calmSuffixes[1]
	}

	return calmPrefixes[0] + response + calmSuffixes[0]
}

// AddWelcomeTone 添加欢迎语调
func AddWelcomeTone(response, emotion string) string {
	welcomePrefixes := []string{"", "欢迎！", "你好呀，", "很高兴认识你，"}
	welcomeSuffixes := []string{"", "，有什么想聊的吗？", "，我们可以慢慢了解", ""}

	if emotion == "中性" && response == "?" {
		return welcomePrefixes[2] + "有什么想聊的吗？"
	}

	return response + welcomeSuffixes[0]
}
