//go:build make_root

package afl

import (
	"crypto/sha512"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-sql-driver/mysql"
)

func init() {
	db.AutoMigrate(&Root{})
	//Note:仅在使用特定构建标签时启用这里，且应当只部署到受信任的内网，
	//所以这里不需要额外的安全措施。
	s.GET("/create_root", func(ctx *gin.Context) {
		ctx.File("./afl_html/create_root.html")
	})
	s.POST("/create_root", func(ctx *gin.Context) {
		r := Root{}
		if err := ctx.Bind(&r); err != nil {
			//TODO:更好的处理
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}
		s := sha512.Sum512([]byte(r.Password))
		r.Password = base64.StdEncoding.EncodeToString(s[:])
		result := db.Create(&r)
		//TODO:处理错误
		if result.Error != nil {
			if mysqlErr, ok := result.Error.(*mysql.MySQLError); ok {
				switch mysqlErr.Number {
				case 1062: // MySQL code for duplicate entry
					ctx.String(409, "用户名已被使用")
					return
				}
			}
			panic(result.Error)
		}
		se := c.NewSession(ctx.ClientIP(), ctx.Request.UserAgent(), r.Name)
		c.SetSession(&se, ctx.Writer)
		r.SessionID = se.ID
		result = db.Save(&r)
		if result.Error != nil {
			panic(result.Error)
		}
		//TODO:返回html
		ctx.String(200, "创建成功")
	})
}
