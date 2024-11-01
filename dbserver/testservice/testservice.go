package testservice

import (
	"common/collect"
	"common/db"
	"math"
	"strconv"
	"time"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
	"go.mongodb.org/mongo-driver/bson"
)

type TestService struct {
	service.Service
}

func init() {
	node.Setup(&TestService{})
}

func (slf *TestService) OnInit() error {
	return nil
}

func (slf *TestService) OnStart() {
	var req db.DBControllerReq
	sort1 := &db.Sort{}
	sort1.SortField = "_id"
	sort1.Asc = false
	req.Sort = append(req.Sort, sort1)
	req.MaxRow = 1
	req.CollectName = collect.AccountCollectName
	req.NotUseCache = true
	req.Type = db.OptType_Find
	req.Key = strconv.FormatInt(time.Now().Unix(), 10)
	//data, _ := bson.Marshal(bson.M{"$inc": bson.M{"UserNum": 1}})
	//req.RawData = append(req.Data, data)
	var userId int64
	var platSrvId int64
	platSrvId |= 1
	platSrvId |= (1 << 12)
	req.Condition, _ = bson.Marshal(bson.D{{"PlatformServerID", platSrvId}})
	account := collect.CAccount{}
	slf.AsyncCall("DBService.RPC_DBRequest", &req, func(res *db.DBControllerRet, err error) {
		if len(res.Res) > 0 {
			err := bson.Unmarshal(res.Res[0], &account)
			if err != nil {
				log.Error(err.Error())
				return
			}
			userId = (account.UserID & 0xffffffff)
			if userId > math.MaxInt32 {
				return
			}
			userId = account.UserID + 1
		} else {
			platSrvId |= 1
			platSrvId |= (int64(1) << 12)
			userId |= (platSrvId << 32)
			userId |= 1
		}

		req = db.DBControllerReq{}
		req.CollectName = collect.AccountCollectName
		req.NotUseCache = true
		req.Type = db.OptType_Insert
		data, _ := bson.Marshal(bson.D{{"_id", userId}, {"PlatformServerID", platSrvId}})
		req.Key = string(data)
		req.Data = append(req.Data, data)
		slf.AsyncCall("DBService.RPC_DBRequest", &req, func(res *db.DBControllerRet, err error) {
			if err != nil {

			}
		})
	})
}
