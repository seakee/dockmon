package collector

import (
	"context"

	collectorModel "github.com/seakee/dockmon/app/model/collector"
	"github.com/seakee/dockmon/app/repository/collector"
	"github.com/sk-pkg/logger"
	"github.com/sk-pkg/redis"
	"gorm.io/gorm"
)

type (
	LogService interface {
		Store(ctx context.Context, log *collectorModel.Log) (int, error)
	}

	logService struct {
		repo   collector.Repo
		logger *logger.Manager
		redis  *redis.Manager
	}
)

func (l logService) Store(ctx context.Context, log *collectorModel.Log) (int, error) {
	return l.repo.CreateLog(log)
}

func NewLogService(db *gorm.DB, redis *redis.Manager, logger *logger.Manager) LogService {
	return &logService{
		repo:   collector.NewLogRepo(db, redis),
		logger: logger,
		redis:  redis,
	}
}
