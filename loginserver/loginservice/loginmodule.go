package loginservice

import (
	"common/collect"
	"common/db"
	"common/proto/msg"
	"common/proto/rpc"
	"common/util"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/service"
	"github.com/duanhf2012/origin/v2/sysservice/httpservice"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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
	ECode     int32
	HisAreaId int32
	UserId    string
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
	if len(result.UserId) > 0 {
		token, _ = util.EncryptToken(result.UserId)
		err = db.MakeUpdateId(collect.AccountCollectName, bson.D{{"$set", bson.D{{"LoginIp", session.GetHeader("X-Real-IP")}, {"Token", token}}}},
			result.UserId, token, &req)
	} else {
		account := collect.CAccount{
			UserId:     primitive.NewObjectID().Hex(),
			RegisterIp: session.GetHeader("X-Real-IP"),
			LoginIp:    session.GetHeader("X-Real-IP"),
		}
		account.UserName = loginInfo.UserName
		account.Password = loginInfo.Password

		account.Token, _ = util.EncryptToken(result.UserId)
		token = account.Token
		result.UserId = account.UserId
		err = db.MakeInsertId(collect.AccountCollectName, &account, account.UserId, &req)
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
			UserId:    result.UserId,
			AreaGate:  login.funcGetAllAreaGateUrl(),
			HisAreaId: result.LastArea,
			Token:     token,
		}
		session.WriteJsonDone(http.StatusOK, &resp)
	})

	if err != nil {
		login.WriteResponseError(session, msg.ErrCode_InterNalError)
		log.Error("AsyncCall DBService.RPC_DBRequest fail %s!", err.Error())
	}
}

func (login *LoginModule) WriteResponseError(session *httpservice.HttpSession, eCode msg.ErrCode) {
	var resp HttpRespone
	resp.ECode = int32(eCode)

	session.WriteJsonDone(http.StatusOK, &resp)
}

func (login *LoginModule) Login(session *httpservice.HttpSession) {
	//1.验证Body请求内容
	var loginInfo rpc.LoginInfo
	body, _ := url.QueryUnescape(string(session.GetBody()))
	err := json.Unmarshal([]byte(body), &loginInfo)
	if err != nil || loginInfo.UserName == "" || loginInfo.Password == "" {
		login.WriteResponseError(session, msg.ErrCode_LoginParamError)
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
	UserId string
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

	userId, err := util.DecryptToken(saveArea.Token)
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
	req.Key = userId

	//存住进最后一次登陆的区服Id
	data, _ := bson.Marshal(bson.M{"$set": bson.M{"LastArea": saveArea.AreaId}})
	req.Data = append(req.Data, data)
	err = login.Go("DBService.RPC_DBRequest", &req)
	if err != nil {
		saveAreaResp.ECode = msg.ErrCode_InterNalError
	}
	session.WriteJsonDone(http.StatusOK, &saveAreaResp)
}
