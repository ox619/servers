package main

import (
	_ "loginserver/loginservice"
	"time"

	"github.com/duanhf2012/origin/v2/node"
)

func main() {
	node.OpenProfilerReport(time.Second * 10)
	node.Start()
}
