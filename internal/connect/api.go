package connect

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"project-yume/internal/model"
	"project-yume/internal/utils"

	"github.com/gorilla/websocket"
)

const apiCallTimeout = 5 * time.Second

var (
	apiResponseWaiters sync.Map // map[string]chan model.APIResponse
)

func CallAPI(conn *websocket.Conn, action string, params interface{}) (model.APIResponse, error) {
	if conn == nil {
		return model.APIResponse{}, fmt.Errorf("websocket connection is nil")
	}

	echo := utils.NewRequestID("api")
	waiter := make(chan model.APIResponse, 1)
	apiResponseWaiters.Store(echo, waiter)
	defer apiResponseWaiters.Delete(echo)

	request := model.Message{
		Action: action,
		Params: params,
		Echo:   echo,
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return model.APIResponse{}, err
	}

	if err := WriteMessage(conn, websocket.TextMessage, payload); err != nil {
		return model.APIResponse{}, err
	}

	timer := time.NewTimer(apiCallTimeout)
	defer timer.Stop()

	select {
	case resp := <-waiter:
		if resp.Status != "" && resp.Status != "ok" {
			return resp, fmt.Errorf("onebot api %s failed: status=%s retcode=%d", action, resp.Status, resp.RetCode)
		}
		if resp.RetCode != 0 {
			return resp, fmt.Errorf("onebot api %s failed: retcode=%d", action, resp.RetCode)
		}
		return resp, nil
	case <-timer.C:
		return model.APIResponse{}, fmt.Errorf("onebot api %s timeout", action)
	}
}

func DispatchAPIResponse(raw []byte) bool {
	var resp model.APIResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false
	}
	if resp.Echo == "" {
		return false
	}

	waiterValue, ok := apiResponseWaiters.Load(resp.Echo)
	if !ok {
		return false
	}

	waiter := waiterValue.(chan model.APIResponse)
	select {
	case waiter <- resp:
	default:
	}
	return true
}
