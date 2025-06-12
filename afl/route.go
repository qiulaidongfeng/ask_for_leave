package afl

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Route(s *gin.Engine) {
	csrf := http.NewCrossOriginProtection()
	s.Use(func(ctx *gin.Context) {
		h := csrf.Handler(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx.Next()
		}))
		h.ServeHTTP(ctx.Writer, ctx.Request)
	})
	s.GET("/", func(ctx *gin.Context) {
		// 如果有还在审批中的审批，不允许发新的申请
		id, err := ctx.Request.Cookie("pending")
		if err != nil {
			ctx.File("./afl_html/index.html")
			return
		}
		u, err := strconv.ParseUint(id.Value, 10, 64)
		if err != nil {
			id.MaxAge = -1
			http.SetCookie(ctx.Writer, id)
			ctx.File("./afl_html/index.html")
			return
		}
		var r = Request{ID: uint(u)}
		if result := db.Model(&Request{}).Find(&r); result.Error != nil {
			panic(result.Error)
		}
		if r.Reviewer == "" {
			var buf bytes.Buffer
			t3.Execute(&buf, u)
			ctx.Data(200, "text/html", buf.Bytes())
			return
		}
		id.MaxAge = -1
		http.SetCookie(ctx.Writer, id)
		ctx.File("./afl_html/index.html")
	})
	s.POST("/request", func(ctx *gin.Context) {
		r := ParserRequest(ctx)
		if r == nil {
			return
		}
		id := r.ToDb()
		http.SetCookie(ctx.Writer, &http.Cookie{
			Name:     "pending",
			Value:    strconv.Itoa(int(id)),
			Secure:   true,
			HttpOnly: true,
		})
		var buf bytes.Buffer
		t3.Execute(&buf, id)
		ctx.Data(200, "text/html", buf.Bytes())
	})
	s.GET("/search", func(ctx *gin.Context) {
		if id := ctx.Query("id"); id != "" {
			search(ctx, id)
			return
		}
		ctx.File("./afl_html/search.html")
	})
	s.POST("/search", func(ctx *gin.Context) {
		id := ctx.PostForm("id")
		search(ctx, id)
	})
	s.GET("/root", func(ctx *gin.Context) {
		s, err := ctx.Request.Cookie("session")
		if err == nil {
			b, err, se := c.CheckLogined(ctx.ClientIP(), ctx.Request.UserAgent(), s)
			if err != nil {
				s.MaxAge = -1
				http.SetCookie(ctx.Writer, s)
				//TODO:返回html
				ctx.String(401, err.Error())
				return
			}
			if b {
				r := GetAll(se.Name)
				var buf bytes.Buffer
				err := t2.Execute(&buf, r)
				if err != nil {
					panic(err)
				}
				ctx.Data(200, "text/html", buf.Bytes())
				return
			}
		}
		ctx.File("./afl_html/login.html")
	})
	s.POST("/root", func(ctx *gin.Context) {
		r := &Root{}
		if err := ctx.Bind(r); err != nil {
			//TODO:更好的处理
			ctx.String(http.StatusBadRequest, err.Error())
			return
		}
		s := sha512.Sum512([]byte(r.Password))
		r.Password = base64.StdEncoding.EncodeToString(s[:])
		result := db.Find(r)
		if result.Error == gorm.ErrRecordNotFound {
			// TODO:返回html
			ctx.String(401, "无此审批者或密码错误")
		} else if result.Error != nil {
			panic(result.Error)
		}
		se := c.NewSession(ctx.ClientIP(), ctx.Request.UserAgent(), r.Name)
		r.SessionID = se.ID
		result = db.Save(r)
		if result.Error != nil {
			panic(result.Error)
		}
		c.SetSession(&se, ctx.Writer)
		ctx.Redirect(302, "/root")
	})
	s.POST("/approve", func(ctx *gin.Context) {
		u, name := pre_op(ctx)
		r := &Request{ID: uint(u)}
		result := db.Model(&r).Updates(&Request{Reviewer: name, Approve: true, ReviewerTime: time.Now().UTC().Add(8 * time.Hour)})
		if result.Error != nil {
			panic(result.Error)
		}
		ctx.Redirect(302, "/root")
	})
	s.POST("/refuse", func(ctx *gin.Context) {
		u, name := pre_op(ctx)
		r := &Request{ID: uint(u)}
		result := db.Model(&r).Updates(&Request{Reviewer: name, Approve: false, ReviewerTime: time.Now().UTC().Add(8 * time.Hour)})
		if result.Error != nil {
			panic(result.Error)
		}
		ctx.Redirect(302, "/root")
	})
}

func search(ctx *gin.Context, id string) {
	u, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		//TODO:更好的处理
		ctx.String(http.StatusBadRequest, "id应该是数字")
		return
	}
	r := Search(uint(u))
	var buf bytes.Buffer
	err = t.Execute(&buf, r)
	if err != nil {
		panic(err)
	}
	ctx.Data(200, "text/html", buf.Bytes())
}

func pre_op(ctx *gin.Context) (uint, string) {
	tmp, err := ctx.Request.Cookie("session")
	if err != nil {
		// TODO:返回html
		ctx.String(http.StatusUnauthorized, "未登录")
		return 0, ""
	}
	b, err, se := c.CheckLogined(ctx.ClientIP(), ctx.Request.UserAgent(), tmp)
	if !b {
		// TODO:返回html
		ctx.String(http.StatusUnauthorized, "未登录")
		return 0, ""
	}
	if err != nil {
		// TODO:返回html
		ctx.String(http.StatusUnauthorized, err.Error())
		return 0, ""
	}
	id := ctx.Query("id")
	u, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		//TODO:更好的处理
		ctx.String(http.StatusBadRequest, "id应该是数字")
		return 0, ""
	}
	return uint(u), se.Name
}
