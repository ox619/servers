package authservice

import (
	"bytes"
	"common/collect"
	"common/db"
	"common/proto/msg"
	"common/proto/rpc"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	node.Setup(&AuthService{})
}

func HttpGet(url string) ([]byte, error) {
	var req *http.Response
	var err error
	req, err = http.Get(url)
	if err != nil {
		return nil, err
	}

	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func HttpPost(url, format string, buf *bytes.Buffer) ([]byte, error) {
	var req *http.Response
	var err error
	req, err = http.Post(url, format, buf)
	if err != nil {
		return nil, err
	}

	defer req.Body.Close()
	resp, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type AuthService struct {
	service.Service
}

func (auth *AuthService) OnInit() error {
	var authCfg struct {
		MinGoroutineNum   int32
		MaxGoroutineNum   int32
		MaxTaskChannelNum int
	}
	err := auth.ParseServiceCfg(&authCfg)
	if err != nil {
		return err
	}

	//2.打开协程模式
	auth.OpenConcurrent(authCfg.MinGoroutineNum, authCfg.MaxGoroutineNum, authCfg.MaxTaskChannelNum)

	//性能监控
	auth.OpenProfiler()
	auth.GetProfiler().SetOverTime(time.Second * 2)
	auth.GetProfiler().SetMaxOverTime(time.Second * 10)

	return nil
}

func (auth *AuthService) RPC_Check(loginInfo *rpc.LoginInfo, loginResult *rpc.LoginResult) error {
	return auth.CheckAccount(loginInfo, loginResult)
}

func (auth *AuthService) CheckAccount(loginInfo *rpc.LoginInfo, loginResult *rpc.LoginResult) error {
	var req db.DBControllerReq
	req.CollectName = collect.AccountCollectName
	req.Type = db.OptType_Find
	req.MaxRow = 1
	req.Condition, _ = bson.Marshal(bson.D{{"Account", loginInfo.Account}})
	req.Key = loginInfo.Account
	ret := db.DBControllerRet{}
	err := auth.GetService().GetRpcHandler().Call("DBService.RPC_DBRequest", &req, &ret)
	if err != nil {
		loginResult.Ret = int32(msg.ErrCode_InterNalError)
		log.Error("Call DBService.RPC_DBRequest fail %s!", err.Error())
		return err
	}

	if len(ret.Res) != 0 {
		account := collect.CAccount{}
		bson.Unmarshal(ret.Res[0], &account)
		if account.Password != "" && account.Password != loginInfo.Password {
			loginResult.Ret = int32(msg.ErrCode_InterNalError)
			log.Warning("check account failed, account %s password %d mismatch", loginInfo.Account, loginInfo.Password)
			return nil
		}
		loginResult.UserID = account.UserID
	}
	return nil
}

func (auth *AuthService) CheckSteam(loginInfo *rpc.LoginInfo, loginResult *rpc.LoginResult) error {
	url := fmt.Sprintf("https://partner.steam-api.com/ISteamUserAuth/AuthenticateUserTicket/v1/?appid=%d&key=%s&ticket=%s",
		1835930, "4C4F543FBD30466FFD9F9CABF85226B2", loginInfo.Ticket)
	respBytes, err := HttpGet(url)
	if err != nil {
		loginResult.Ret = int32(msg.ErrCode_InterNalError)
		return err
	}

	type Params struct {
		Result  string `json:"result"`
		SteamID string `json:"steamid"`
	}

	type Response struct {
		Params Params `json:"params"`
	}
	resp := struct {
		Response Response `json:"response"`
	}{}

	err = json.Unmarshal(respBytes, &resp)
	if err != nil {
		log.Error("check steam failed, %s", string(respBytes))
		loginResult.Ret = int32(msg.ErrCode_InterNalError)
		return err
	}
	if resp.Response.Params.Result != "OK" {
		loginResult.Ret = int32(msg.ErrCode_InterNalError)
		log.Error("check steam failed, resp:%+v, response:%s", resp, respBytes)
		return fmt.Errorf("steam check failed")
	}

	loginResult.SteamID = resp.Response.Params.SteamID

	var req db.DBControllerReq
	req.CollectName = collect.AccountCollectName
	req.Type = db.OptType_Find
	req.MaxRow = 1
	req.Condition, _ = bson.Marshal(bson.D{{"Account", loginResult.SteamID}})
	req.Key = loginResult.SteamID
	ret := db.DBControllerRet{}
	err = auth.GetService().GetRpcHandler().Call("DBService.RPC_DBRequest", &req, &ret)
	if err != nil {
		loginResult.Ret = int32(msg.ErrCode_InterNalError)
		log.Error("Call DBService.RPC_DBRequest fail %s!", err.Error())
		return err
	}

	if len(ret.Res) != 0 {
		account := collect.CAccount{}
		bson.Unmarshal(ret.Res[0], &account)
		loginResult.UserID = account.UserID
	}
	return nil
}
