package push

import (
	"path/filepath"
)

// InitAllPushers 统一初始化所有推送厂商并注册到工厂
// baseDir 为证书存放在 scripts/v1/ 的基础路径
func InitAllPushers(baseDir string) {
	// 小米
	Register(0, NewXiaomiPusher(filepath.Join(baseDir, "xiaomi.json")))

	// 华为
	Register(1, NewHuaweiPusher(filepath.Join(baseDir, "huawei.json")))

	// oppo
	Register(2, NewOppoPusher(filepath.Join(baseDir, "oppo.json")))

	// vivo
	Register(3, NewVivoPusher(filepath.Join(baseDir, "vivo.json")))

	// 荣耀
	Register(4, NewHonorPusher(filepath.Join(baseDir, "honor.json")))

	// 魅族
	Register(5, &NoopPusher{BrandName: "Meizu"})

	// ios
	Register(6, NewIosPusher(filepath.Join(baseDir, "ios.json")))

	// 鸿蒙
	Register(7, NewHarmonyPusher(filepath.Join(baseDir, "harmony.json")))

	// 三星
	Register(8, &NoopPusher{BrandName: "Samsung"})

	// 其他
	Register(9, &NoopPusher{BrandName: "Other"})
}
