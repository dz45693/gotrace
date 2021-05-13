package model

import (
	"context"
	"github.com/jinzhu/gorm"
	"tracedemo/db"
)

type UserInfo struct {
	ID int32 `gorm:"primary_key;column:id;type:int(11) AUTO_INCREMENT;not null;" json:"id"`
	Name string `gorm:"column:name;type:varchar(50);" json:"name"`
	Hobby string `gorm:"column:hobby;type:varchar(50);" json:"hobby"`
}

func (u *UserInfo) TableName()string  {
	return "userInfo"
}

func (u *UserInfo) Create(gormDb *gorm.DB) error  {
	return gormDb.Create(u).Error
}

func (u *UserInfo) Update(gormDb *gorm.DB) error  {
	return gormDb.Save(u).Error
}

func (u *UserInfo) Delete(gormDb *gorm.DB) error  {
	return gormDb.Delete(u).Error
}

func GetAllUser(ctx context.Context)([]*UserInfo,error)  {
	var ret []*UserInfo

	gormDb:=db.GetMaster(ctx)
	err:=gormDb.Table(new(UserInfo).TableName()).Find(&ret).Error

	return ret,err
}
