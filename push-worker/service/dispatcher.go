package service

import (
	"context"
	"fmt"
	"push-worker/global"
	"push-worker/models"
	"push-worker/push"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/nats-io/nats.go"
	"github.com/redis/rueidis"
)

func handleTask(msg *nats.Msg) {
	var data models.CommonMessage
	if err := sonic.Unmarshal(msg.Data, &data); err != nil {
		msg.Ack()
		return
	}

	ctx := context.Background()
	// 1. 去重逻辑 (仍然使用 Redis，因为是分布式多节点)
	lockKey := fmt.Sprintf("lock:%d:%s", data.UserID, data.Sn)
	if err := global.REDIS.Do(ctx, global.REDIS.B().Set().Key(lockKey).Value("1").Nx().Ex(10*time.Second).Build()).Error(); rueidis.IsRedisNil(err) {
		msg.Ack()
		return
	}

	// 2. 多级查询用户 (Redis CSC -> MySQL)
	var user models.UserSession
	sessionKey := fmt.Sprintf("user:%d", data.UserID)
	// 利用 rueidis 的客户端缓存减少网络损耗
	val, err := global.REDIS.DoCache(ctx, global.REDIS.B().Get().Key(sessionKey).Cache(), 5*time.Minute).ToString()
	if err != nil {
		query := "SELECT user_id, notify_enable,refresh_token, access_token, device_token, brand, update_time FROM t_user_session WHERE user_id = ?"
		if err := global.DB.GetContext(ctx, &user, query, data.UserID); err != nil {
			msg.Ack()
			return
		}
		// 异步回写
		go func(u models.UserSession) {
			j, _ := sonic.MarshalString(u)
			global.REDIS.Do(context.Background(), global.REDIS.B().Set().Key(sessionKey).Value(j).Ex(10*time.Minute).Build())
		}(user)
	} else {
		sonic.UnmarshalString(val, &user)
	}

	if *user.NotifyEnable == false {
		msg.Ack()
		return
	}
	// 不支持魅族、三星、和其他厂商的推送
	if user.Brand == "5" || user.Brand == "8" || user.Brand == "9" {
		msg.Ack()
		return
	}
	// 转化格式
	brandInt, err := strconv.Atoi(user.Brand)
	if err != nil {
		msg.Ack()
		return
	}

	// 3. 调度推送实现
	if handler := push.GetHandler(brandInt); handler != nil {
		_ = handler.Send(ctx, user.DeviceToken, data)
	}
	msg.Ack()
}
