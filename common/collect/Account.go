package collect

var AccountCollectName = "Account"

type CAccount struct {
	UserID           int64  `bson:"_id"`              //生成最新id
	Account          string `bson:"Account"`          //账号
	Password         string `bson:"Password"`         //密码
	PlatformServerID uint8  `bson:"PlatformServerID"` //平台id 8位|服务器id 12位
	RegisterIP       string `bson:"RegisterIP"`       //注册IP
	LoginIP          string `bson:"LoginIP"`          //登录IP
	Token            string `bson:"Token"`            //Token
}
