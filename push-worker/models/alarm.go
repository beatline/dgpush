package models

// 1. 定义枚举类型 (底层为 int)
type Alarm int

// 2. 定义枚举常量
const (
	AlarmMotion    Alarm = 0
	AlarmPerson    Alarm = 1
	AlarmFire      Alarm = 2
	AlarmSmoke     Alarm = 3
	AlarmFireworks Alarm = 4
	AlarmPR        Alarm = 5
	AlarmVehicle   Alarm = 7
	AlarmPet       Alarm = 8
	AlarmOther     Alarm = -1
)

// 3. 定义映射表 (Code -> Description)
// 使用 map 可以提供 O(1) 的查询速度
var alarmDescMap = map[Alarm]string{
	AlarmMotion:    "画面变化",
	AlarmPerson:    "人形报警",
	AlarmFire:      "火焰报警",
	AlarmSmoke:     "烟雾报警",
	AlarmFireworks: "烟火报警",
	AlarmPR:        "人形报警",
	AlarmVehicle:   "车辆报警",
	AlarmPet:       "宠物报警",
	AlarmOther:     "画面变化",
}

// GetAlarmDesc 是你要求的对外暴露的方法
// 逻辑：传入 int 值，返回对应的描述字符串。如果不匹配，返回默认值(Other)
func GetAlarmDesc(code int) string {
	// 将 int 转换为 Alarm 类型进行查找
	alarm := Alarm(code)

	// 查找 map
	if desc, ok := alarmDescMap[alarm]; ok {
		return desc
	}

	// 如果没找到，返回默认值 (对应 Java 代码中的 return Alarm.OTHER)
	return alarmDescMap[AlarmOther]
}

// 扩展建议：实现 String() 方法
// 这样 fmt.Println(AlarmFire) 会直接输出 "火焰报警" 而不是 2
func (a Alarm) String() string {
	return GetAlarmDesc(int(a))
}
