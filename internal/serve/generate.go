package serve

import (
	"log"
	"math/rand"
	"time"

	"project-yume/internal/aifunction"
	"project-yume/internal/global"
)

var respToDealSad = []string{
	"别难过了，开心点",
	"开心点嘛",
}

func Generate(t string) (msg string, err error) {
	if t == "sad" {
		return GenerateInSadReq(), nil
	}
	return "", nil
}

func GenerateInSadReq() string {
	// 生成随机数
	rand.New(rand.NewSource(time.Now().UnixNano()))
	idx := rand.Intn(len(respToDealSad) + 1)
	if idx == len(respToDealSad) {
		newmsg, err := aifunction.Queryai(global.Prompt, "我好难过，安慰我一下")
		if err != nil {
			log.Print("请求ai失败 in GenerateInSadReq:", err)
			return ""
		}
		return newmsg
	}
	return respToDealSad[idx]
}
