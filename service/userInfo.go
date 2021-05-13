package service

import (
	"context"
	"tracedemo/db"
	"tracedemo/logger"
	"tracedemo/model"
)

func TestUserInfo(ctx context.Context) error {
	info := model.UserInfo{
		Name:  "gavin",
		Hobby: "demo",
	}

	gormDb := db.GetMaster(ctx)

	err := info.Create(gormDb)
	if err != nil {
		logger.Error(ctx, "Create err %v", err)
	} else {
		logger.Info(ctx, "create Success!")
	}

	//update
	info.Name = "test"
	err = info.Update(gormDb)
	if err != nil {
		logger.Warn(ctx, "Update err %v", err)
	} else {
		logger.Debug(ctx, "Update Success!")
	}

	//get
	infos, err := model.GetAllUser(ctx)
	if err != nil {
		logger.Warn(ctx, "Get err %v", err)
	} else {
		logger.Debug(ctx, "Get Success %v", len(infos))
	}

	//delete
	err = info.Delete(gormDb)
	if err != nil {
		logger.Error(ctx, "Delete err %v", err)
	} else {
		logger.Info(ctx, "Delete Success!")
	}

	return nil
}
