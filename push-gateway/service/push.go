package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/json"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
	"push-gateway/global"
	"push-gateway/models"
	"strconv"
	"sync"
	"time"
)

const (
	SubjectPushCommon = "push.task.common"
	SubjectPushAlarm  = "push.task.alarm"
)

// 全局singleflight实例
var g singleflight.Group

// 对象池：减少 GC 压力
var commonMsgPool = sync.Pool{
	New: func() interface{} { return &models.CommonMessage{} },
}

// dispatch 内部通用调度函数（高性能风格）
func dispatch(subject string, data *models.CommonMessage) {
	// 1. 立即序列化（由于对象是从池子里借的，必须在归还前完成序列化）
	payload, err := sonic.Marshal(data)

	// 2. 序列化完成，立即归还对象到池中
	commonMsgPool.Put(data)

	if err != nil {
		global.LOG.Error("消息序列化失败", zap.Error(err))
		return
	}

	// 3. 提交到 ants 协程池进行 NATS 异步发布，不阻塞主流程
	_ = global.MsgPool.Submit(func() {
		_, err := global.JS.PublishAsync(subject, payload)
		if err != nil {
			global.LOG.Error("NATS发布失败", zap.String("subject", subject), zap.Error(err))
		}
	})
}

// PushMessage 发送自定义消息 (1k QPS)
func PushMessage(c context.Context, ctx *app.RequestContext) {
	var req models.CommonMessage
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(400, utils.H{"code": 400, "message": "参数错误"})
		return
	}

	// 从池中获取并赋值
	msg := commonMsgPool.Get().(*models.CommonMessage)
	*msg = req // 结构体直接赋值（浅拷贝），效率极高

	dispatch(SubjectPushCommon, msg)
	ctx.JSON(200, utils.H{"code": 200, "message": "success"})
}

// PushAlarmMessage 发送告警消息 (10k QPS)
func PushAlarmMessage(c context.Context, ctx *app.RequestContext) {
	var alarm models.AlarmMessage
	if err := ctx.BindAndValidate(&alarm); err != nil {
		ctx.JSON(400, utils.H{"code": 400, "message": "参数错误"})
		return
	}

	// 从池中获取并手动转换（补全业务逻辑默认值）
	msg := commonMsgPool.Get().(*models.CommonMessage)
	msg.UserID = alarm.UserID
	msg.Sn = alarm.Sn
	msg.Image = alarm.Image
	msg.Extra = alarm.Extra
	msg.ActionType = 0    // 告警业务默认值
	msg.ActionIntent = "" // 告警业务默认值
	alarmInt, err := strconv.Atoi(alarm.Alarm)
	var alarmDesc string
	if err != nil {
		fmt.Println("转换失败:", err)
		// 转化失败，使用默认值
		alarmDesc = alarm.Alarm
	} else {
		// 获取告警描述文本
		alarmDesc = models.GetAlarmDesc(alarmInt)
	}
	if alarm.Title == "" || alarm.Body == "" {
		msg.Title = "检测到" + alarmDesc
		msg.Body = "设备" + alarm.Sn + "检测到有" + alarmDesc + "，请及时处理。"
	} else {
		msg.Title = alarm.Title
		msg.Body = alarm.Body
	}
	dispatch(SubjectPushAlarm, msg)

	ctx.JSON(200, utils.H{"code": 200, "message": "success"})
}

// GetPushStatus 获取用户推送状态
func GetPushStatus(c context.Context, ctx *app.RequestContext) {
	var req models.StatusRequest
	if err := ctx.BindAndValidate(&req); err != nil {
		ctx.JSON(consts.StatusBadRequest, utils.H{"code": 400, "message": "参数错误"})
		return
	}

	key := fmt.Sprintf("push:session:%d", req.UserID)

	// 从 Redis 获取
	resp := global.REDIS.Do(c, global.REDIS.B().Get().Key(key).Build())
	if val, err := resp.ToString(); err == nil {
		// 如果 Redis 存在 NULL 占位符，说明 MySQL 确实没数据，直接返回 404
		if val == "NULL" {
			ctx.JSON(consts.StatusNotFound, utils.H{"code": 404, "message": "用户不存在"})
			return
		}
		var session models.UserSession
		if json.Unmarshal([]byte(val), &session) == nil {
			ctx.JSON(consts.StatusOK, utils.H{"code": 200, "notify_enable": session.NotifyEnable})
			return
		}
	}

	// 2. Redis 未命中，使用 singleflight 合并 MySQL 回源请求
	// 保证同一时刻只有一个请求去查 MySQL，其余请求等待结果
	result, err, _ := g.Do(fmt.Sprintf("status:%d", req.UserID), func() (interface{}, error) {
		var session models.UserSession
		// 同步查询 MySQL 全量数据
		query := "SELECT user_id, notify_enable,refresh_token, access_token, device_token, brand, update_time FROM t_user_session WHERE user_id = ?"
		err := global.DB.GetContext(c, &session, query, req.UserID)
		if err != nil {
			return nil, err
		}

		// MySQL 查询成功，同步写入 Redis 缓存
		cacheData, _ := json.Marshal(session)
		global.REDIS.Do(context.Background(),
			global.REDIS.B().Set().Key(key).Value(string(cacheData)).Ex(1*time.Hour).Build())

		return session.NotifyEnable, nil
	})

	// 3. 错误处理（严格遵循业务逻辑）
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// MySQL 没数据：设置空缓存防止穿透，并返回 404 报错
			global.REDIS.Do(context.Background(),
				global.REDIS.B().Set().Key(key).Value("NULL").Ex(15*time.Minute).Build())
			ctx.JSON(consts.StatusNotFound, utils.H{"code": 404, "message": "用户不存在"})
			return
		}
		// 数据库连接等其他错误
		global.LOG.Error("查询数据库失败", zap.Error(err))
		ctx.JSON(consts.StatusInternalServerError, utils.H{"code": 500, "message": "服务繁忙"})
		return
	}

	// 4. 返回准确的状态
	notifyEnable := result.(*bool)
	ctx.JSON(consts.StatusOK, utils.H{"code": 200, "notify_enable": notifyEnable})
}

// SetPushStatus 更新用户推送状态
func SetPushStatus(c context.Context, ctx *app.RequestContext) {
	var req models.StatusRequest
	if err := ctx.BindAndValidate(&req); err != nil || req.NotifyEnable == nil {
		ctx.JSON(consts.StatusBadRequest, utils.H{"code": 400, "message": "参数错误"})
		return
	}

	// 更新数据库
	enableInt := map[bool]int{true: 1, false: 0}[*req.NotifyEnable]
	query := "UPDATE t_user_session SET notify_enable = ?, update_time = NOW() WHERE user_id = ?"
	res, err := global.DB.ExecContext(c, query, enableInt, req.UserID)
	if err != nil {
		global.LOG.Error("更新数据库失败", zap.Error(err))
		ctx.JSON(consts.StatusInternalServerError, utils.H{"code": 500, "message": "fail"})
		return
	}

	// 判断用户是否存在
	count, _ := res.RowsAffected()
	if count == 0 {
		ctx.JSON(consts.StatusNotFound, utils.H{"code": 404, "message": "用户不存在"})
		return
	}

	// 3. 保证一致性的关键：直接删除缓存
	key := fmt.Sprintf("user:session:%d", req.UserID)
	global.REDIS.Do(c, global.REDIS.B().Del().Key(key).Build())

	// 4. 前端只关心成功
	ctx.JSON(consts.StatusOK, utils.H{"code": 200, "message": "success"})
}
