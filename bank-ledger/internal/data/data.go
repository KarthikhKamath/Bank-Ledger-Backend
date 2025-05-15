package data

import (
	"bank-ledger/internal/conf"
	"bank-ledger/internal/entity"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

var ProviderSet = wire.NewSet(NewData, NewMongoDBConnection, NewAccountRepo, NewTransactionRepo, NewTransactionLogsRepo)

type Data struct {
	db  *gorm.DB
	log *log.Helper
}

func NewData(c *conf.Data, logger log.Logger) (*Data, func(), error) {
	helper := log.NewHelper(logger)
	var db *gorm.DB
	var err error

	if c.Database.Driver == "mysql" {
		dsn := c.Database.Source
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect to database: %w", err)
		}

		err = db.AutoMigrate(&entity.Account{}, &entity.Transaction{})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to auto-migrate: %w", err)
		}
	} else {
		return nil, nil, fmt.Errorf("unsupported database driver: %s", c.Database.Driver)
	}

	cleanup := func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		helper.Info("closing the data resources")
	}

	return &Data{db: db, log: helper}, cleanup, nil
}

func (d *Data) DB() *gorm.DB {
	return d.db
}
