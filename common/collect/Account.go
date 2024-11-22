package collect

var AccountCollectName = "Account"

type CAccount struct {
	UserId     string `bson:"_id"`        //生成最新id
	UserName   string `bson:"UserName"`   //账号
	Password   string `bson:"Password"`   //密码
	LastServer int32  `bson:"LastServer"` //平台id 8位|服务器id 12位
	RegisterIp string `bson:"RegisterIp"` //注册IP
	LoginIp    string `bson:"LoginIp"`    //登录IP
	Token      string `bson:"Token"`      //Token
}
