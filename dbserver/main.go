package main

import (
	_ "dbserver/dbservice"
	_ "dbserver/testservice"
	"time"

	"github.com/duanhf2012/origin/v2/node"
)

func main() {
	node.OpenProfilerReport(time.Second * 10)
	node.Start()
}
