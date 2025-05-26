package global

import "log"

func ExplainStatus() {
	log.Print(`
		当前状态:
		1. Needcomfort: ` + string(CheckStatus(Needcomfort)) + `
		2. Needencourage: ` + string(CheckStatus(Needencourage)) + `
		3. LongChainflag: ` + string(CheckStatus(LongChainflag)) + `
		4. PerfunctoryFlag: ` + string(CheckStatus(PerfunctoryFlag)) + `
		5. Encourageflag: ` + string(CheckStatus(Encourageflag)) + `
		6. Comfortflag: ` + string(CheckStatus(Comfortflag)) + `
	`)
}

func CheckStatus(status bool) string {
	if status {
		return "1"
	} else {
		return "0"
	}
}
