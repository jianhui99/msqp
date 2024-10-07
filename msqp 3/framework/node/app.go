package node

import (
	"common/logs"
	"encoding/json"
	"framework/remote"
)

// App 就是nats的客户端 处理实际游戏逻辑的服务
type App struct {
	remoteCli remote.Client
	readChan  chan []byte
	writeChan chan *remote.Msg
	handlers  LogicHandler
}

func Default() *App {
	return &App{
		readChan:  make(chan []byte, 1024),
		writeChan: make(chan *remote.Msg, 1024),
		handlers:  make(LogicHandler),
	}
}

func (a *App) Run(serverId string) error {
	a.remoteCli = remote.NewNatsClient(serverId, a.readChan)
	err := a.remoteCli.Run()
	if err != nil {
		return err
	}
	go a.readChanMsg()
	go a.writeChanMsg()
	return nil
}

func (a *App) readChanMsg() {
	//收到的是 其他nas client发送的消息
	for {
		select {
		case msg := <-a.readChan:
			var remoteMsg remote.Msg
			json.Unmarshal(msg, &remoteMsg)
			session := remote.NewSession(a.remoteCli, &remoteMsg)
			session.SetData(remoteMsg.SessionData)
			//根据路由消息 发送给对应的handler进行处理
			router := remoteMsg.Router
			if handlerFunc := a.handlers[router]; handlerFunc != nil {
				result := handlerFunc(session, remoteMsg.Body.Data)
				message := remoteMsg.Body
				var body []byte
				if result != nil {
					body, _ = json.Marshal(result)
				}
				message.Data = body
				//得到结果了 发送给connector
				responseMsg := &remote.Msg{
					Src:  remoteMsg.Dst,
					Dst:  remoteMsg.Src,
					Body: message,
					Uid:  remoteMsg.Uid,
					Cid:  remoteMsg.Cid,
				}
				a.writeChan <- responseMsg
			}
		}
	}

}

func (a *App) writeChanMsg() {
	for {
		select {
		case msg, ok := <-a.writeChan:
			if ok {
				marshal, _ := json.Marshal(msg)
				err := a.remoteCli.SendMsg(msg.Dst, marshal)
				if err != nil {
					logs.Error("app remote send msg err:%v", err)
				}
			}
		}
	}
}

func (a *App) Close() {
	if a.remoteCli != nil {
		a.remoteCli.Close()
	}
}

func (a *App) RegisterHandler(handler LogicHandler) {
	a.handlers = handler
}
