package models

// CommonMessage 标准消息体
type CommonMessage struct {
	UserID       int64  `json:"user_id" vd:"$ > 0"`  // 用户ID
	Sn           string `json:"sn" vd:"len($)>0"`    // 设备sn号
	Title        string `json:"title" vd:"len($)>0"` // 标题
	Body         string `json:"body" vd:"len($)>0"`  // 内容
	Image        string `json:"image"`               // 图片URL
	Extra        string `json:"extra"`               // 消息负载
	ActionType   int64  `json:"action_type"`         // 动作类型 0无动作（打开app首页） 1自定义uri 2自定义url
	ActionIntent string `json:"action_intent"`       // 动作意图
}

// AlarmMessage 告警消息体
type AlarmMessage struct {
	UserID int64  `json:"user_id" vd:"$ > 0"`  // 用户ID
	Sn     string `json:"sn" vd:"len($)>0"`    // 设备sn号
	Alarm  string `json:"alarm" vd:"len($)>0"` // 告警类型 0无报警，1人形报警，2火焰报警，3烟雾报警，4烟火报警，5pr人形报警，6保留类型 7车辆报警，8宠物报警
	Title  string `json:"title" vd:"len($)>0"` // 标题
	Body   string `json:"body" vd:"len($)>0"`  // 内容
	Image  string `json:"image"`               // 图片URL
	Extra  string `json:"extra"`               // 消息负载
}
