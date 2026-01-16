package models

import (
	"time"
)

// UserSession 对应 t_user_session 表结构
type UserSession struct {
	UserID       int64     `json:"user_id" db:"user_id"`             // 用户ID
	NotifyEnable *bool     `json:"notify_enable" db:"notify_enable"` //是否开启通知
	RefreshToken string    `json:"refresh_token" db:"refresh_token"` // 刷新TOKEN
	AccessToken  string    `json:"access_token" db:"access_token"`   //每次请求API的TOKEN
	DeviceToken  string    `json:"device_token" db:"device_token"`   //用于通知的Device Token
	Brand        string    `json:"brand" db:"brand"`                 //手机厂商 0=小米 1=华为 2=oppo 3=vivo 4=荣耀 5=魅族 6=apple 7=鸿蒙 8=三星 9=其他
	UpdateTime   time.Time `json:"update_time" db:"update_time"`     //最后的活跃时间
}

// StatusRequest 接口请求参数
type StatusRequest struct {
	UserID       int64 `json:"user_id" query:"user_id" vd:"$ > 0"` // 用户ID
	NotifyEnable *bool `json:"notify_enable"`                      //是否开启通知
}
