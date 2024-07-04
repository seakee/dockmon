package job

import (
	"github.com/seakee/dockmon/app/job/monitor"
	"github.com/seakee/dockmon/app/pkg/schedule"
	"github.com/sk-pkg/feishu"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

func Register(logger *logger.Manager, redis map[string]*redis.Manager, db map[string]*gorm.DB, feishu *feishu.Manager, s *schedule.Schedule) {
	// Monitor broadband public network IP changes
	ipMonitor := monitor.NewIpMonitor(logger, redis["dockmon"])
	s.AddJob("IpMonitor", ipMonitor).PerMinuit(5).WithoutOverlapping()
}
