package loginservice

import (
	"common/collect"
	"common/db"
	"common/proto/msg"
	"common/proto/rpc"
	"common/util"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/service"
	"github.com/duanhf2012/origin/v2/sysservice/httpservice"
	"go.mongodb.org/mongo-driver/bson"
)

type GateInfoResp struct {
	Weight int32
	Url    string
}

type LoginModule struct {
	service.Module
	seed                  int
	funcGetAllAreaGateUrl func() string
}

func (login *LoginModule) OnInit() error {
	return nil
}

func (login *LoginModule) OnRelease() {
}

type HttpRespone struct {
	ECode     int
	UserId    uint64
	HisAreaId int32
	Token     string
	AreaGate  string
}

func (login *LoginModule) loginCheck(session *httpservice.HttpSession, loginInfo rpc.LoginInfo) {
	err := login.GetService().GetRpcHandler().AsyncCall("AuthService.RPC_Check", &loginInfo, func(loginResult *rpc.LoginResult, err error) {
		if err != nil {
			log.Error("call AuthService.RPC_Check fail %s", err.Error())
			login.WriteResponseError(session, msg.ErrCode_InterNalError)
			return
		}

		if loginResult.Ret != 0 {
			log.Warning("AuthService.RPC_Check fail Ret:%d", loginResult.Ret)
			login.WriteResponseError(session, msg.ErrCode(loginResult.Ret))
			return
		}

		login.loginToDB(session, loginInfo, loginResult)
	})

	//3.服务内部错误
	if err != nil {
		login.WriteResponseError(session, msg.ErrCode_InterNalError)
		log.Error("AsyncCall AuthService.RPC_Check fail %s", err.Error())
	}
}

func (login *LoginModule) loginToDB(session *httpservice.HttpSession, loginInfo rpc.LoginInfo, result *rpc.LoginResult) {
	var req db.DBControllerReq
	var err error
	var token string
	now := time.Now().Unix()
	if result.UserID > 0 {
		token, _ = util.EncryptToken(strconv.FormatUint(result.UserID, 10))
		err = db.MakeUpdateId(collect.AccountCollectName, bson.D{{"$set", bson.D{{"LoginIP", session.GetHeader("X-Real-IP")}, {"Token", token}}}},
			result.UserID, token, &req)
	} else {
		req.CollectName = "AccountSum"
		req.Type = db.OptType_Upset
		req.Condition, _ = bson.Marshal(bson.D{{Key: "Region", Value: 0}, {"Platform", 0}, {"ServerId", 0}})
		req.Key = session.GetHeader("X-Real-IP")
		data, _ := bson.Marshal(bson.M{"$inc": bson.M{"UserNum": 1}})
		req.RawData = append(req.Data, data)
		if err != nil {

		}
	}

	if err != nil {
		log.Error("Create DBService.RPC_DBRequest fail %d!", err.Error())
		login.WriteResponseError(session, msg.ErrCode_InterNalError)
		return
	}

	err = login.GetService().GetRpcHandler().AsyncCall("DBService.RPC_DBRequest", &req, func(res *db.DBControllerRet, err error) {
		if err != nil {
			log.Error("Call DBService.RPC_DBRequest fail %d!", err.Error())
			login.WriteResponseError(session, msg.ErrCode_InterNalError)
			return
		}

		resp := HttpRespone{
			UserId:    result.UserID,
			AreaGate:  login.funcGetAllAreaGateUrl(),
			HisAreaId: result.LastArea,
			Token:     token,
		}
		session.WriteJsonDone(http.StatusOK, &resp)

		if result.Register > 0 {
			interval := util.GetIntervalDay(result.LoginAt, now)
			if interval > 0 {
				collect.UpdateStats(uint8(loginInfo.Channel), collect.Stats_Field_DAU, nil, login.GetService().GetName())
				interval = util.GetIntervalDay(result.Register, now)
				if interval > 0 && interval <= 7 {
					if interval <= 1 {
						collect.UpdateStats(uint8(loginInfo.Channel), collect.Stats_Field_DR1, nil, login.GetService().GetName())
					} else if interval <= 3 {
						collect.UpdateStats(uint8(loginInfo.Channel), collect.Stats_Field_DR3, nil, login.GetService().GetName())
					} else {
						collect.UpdateStats(uint8(loginInfo.Channel), collect.Stats_Field_DR7, nil, login.GetService().GetName())
					}
				}
			}

		} else {
			collect.UpdateStats(uint8(loginInfo.Channel), collect.Stats_Field_Register, nil, login.GetService().GetName())
		}
	})

	if err != nil {
		login.WriteResponseError(session, msg.ErrCode_InterNalError)
		log.Error("AsyncCall DBService.RPC_DBRequest fail %s!", err.Error())
	}
}

func (login *LoginModule) WriteResponseError(session *httpservice.HttpSession, eCode msg.ErrCode) {
	var resp HttpRespone
	resp.ECode = int(eCode)

	session.WriteJsonDone(http.StatusOK, &resp)
}

func (login *LoginModule) Login(session *httpservice.HttpSession) {
	//1.验证Body请求内容
	var loginInfo rpc.LoginInfo
	body, _ := url.QueryUnescape(string(session.GetBody()))
	err := json.Unmarshal([]byte(body), &loginInfo)
	if err != nil || (loginInfo.Channel == rpc.Channel_PC && (loginInfo.UserName == "" || loginInfo.Password == "")) ||
		(loginInfo.Channel == rpc.Channel_Steam && loginInfo.Ticket == "") {
		login.WriteResponseError(session, msg.ErrCode_ParamInvalid)
		log.Warning("The body content of the HTTP request is incorrect:%s!", string(session.GetBody()))
		return
	}

	//2.平台登陆验证
	login.loginCheck(session, loginInfo)
}

type SaveAreaResp struct {
	ECode msg.ErrCode
}

type SaveArea struct {
	UserId uint64
	Token  string
	AreaId int
}

func (login *LoginModule) SaveArea(session *httpservice.HttpSession) {
	var saveAreaResp SaveAreaResp
	var saveArea SaveArea
	body, _ := url.QueryUnescape(string(session.GetBody()))
	err := json.Unmarshal([]byte(body), &saveArea)
	if err != nil || saveArea.Token == "" || saveArea.AreaId <= 0 {
		saveAreaResp.ECode = msg.ErrCode_TokenError
		session.WriteJsonDone(http.StatusOK, &saveAreaResp)
		log.Warning("The body content of the HTTP request is incorrect:%s!", string(session.GetBody()))
		return
	}

	userId, err := global.DecryptToken(saveArea.Token)
	if err != nil || userId != saveArea.UserId {
		saveAreaResp.ECode = msg.ErrCode_TokenError
		session.WriteJsonDone(http.StatusOK, &saveAreaResp)
		log.Warning("The body content of the HTTP request is incorrect:%s!", string(session.GetBody()))
		return
	}

	var req db.DBControllerReq
	req.CollectName = collect.AccountCollectName
	req.Type = db.OptType_Update
	req.Condition, _ = bson.Marshal(bson.D{{"_id", userId}})
	req.Key = 0

	//存住进最后一次登陆的区服Id
	data, _ := bson.Marshal(bson.M{"$set": bson.M{"LastArea": saveArea.AreaId}})
	req.Data = append(req.Data, data)
	err = login.Go("DBService.RPC_DBRequest", &req)
	if err != nil {
		saveAreaResp.ECode = msg.ErrCode_InterNalError
	}
	session.WriteJsonDone(http.StatusOK, &saveAreaResp)
}

func (login *LoginModule) GenToken(userId uint64) {
	origToken := fmt.Sprintf("%d_%d", userId, time.Now().Unix())
	util.AesEncrypt(origToken, global.TokenKey)
}
