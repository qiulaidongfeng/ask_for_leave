package main

import (
	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/ask_for_leave/afl"
)

var s = gin.Default()

func main() {
	afl.Route(s)
	s.RunTLS(":443", "cert.pem", "key.pem")
}
