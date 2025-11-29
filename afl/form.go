package afl

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qiulaidongfeng/key"
	"github.com/qiulaidongfeng/safesession"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Request struct {
	ID         uint `gorm:"primaryKey,autoIncrement"`
	Created    time.Time
	Name       string    `form:"name" binding:"required"`
	Department string    `form:"department" binding:"required"`
	Reason     string    `form:"reason" binding:"required"`
	Date       time.Time `form:"date" time_format:"2006-01-02" binding:"required"`

	Reviewer     string
	Approve      bool
	ReviewerTime time.Time `gorm:"<-:update"`
}

type Count struct {
	Total, Approved, Pending, Refuse int
}

type Root struct {
	Name string `form:"name" gorm:"primaryKey" binding:"required"`
	// Password 是审批者的密码
	// 保存 sha512 哈希值。
	Password  string `form:"password" binding:"required"`
	SessionID string
}

// Review 保存一个审批者可以看到的所有信息
type Review struct {
	Root
	Count        Count
	Pending, All []*Request
}

func ParserRequest(ctx *gin.Context) *Request {
	var ret Request
	if err := ctx.Bind(&ret); err != nil {
		//TODO:更好的处理
		ctx.String(http.StatusBadRequest, err.Error())
		return nil
	}
	return &ret
}

var db = func() *gorm.DB {
	v = newv()
	user, password, addr := GetDsnInfo()
	dsn := user + ":" + password + "@tcp(" + addr + ")/afl?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	err = db.AutoMigrate(&Request{}, &Root{})
	if err != nil {
		panic(err)
	}
	return db
}()

func (r *Request) ToDb() uint {
	//TODO:随机生成ID
	r.Created = time.Now().UTC().Add(8 * time.Hour)
	result := db.Create(r)
	if result.Error != nil {
		panic(result.Error)
	}
	return r.ID
}

func Search(id uint) *Request {
	var ret Request
	ret.ID = id
	result := db.First(&ret)
	if result.Error == gorm.ErrRecordNotFound {
		return nil
	}
	if result.Error != nil {
		panic(result.Error)
	}
	return &ret
}

var t = func() *template.Template {
	b, err := os.ReadFile("./afl_html/search.temp")
	if err != nil {
		panic(err)
	}
	t := template.New("")
	t = t.Funcs(template.FuncMap{
		"getReviewResult": func(r *Request) ReviewResult {
			if r.Reviewer == "" {
				return ReviewResult{State: "未审批", State_css: "status-pending"}
			}
			if r.Approve {
				return ReviewResult{State: "通过", State_css: "status-approved"}
			}
			return ReviewResult{State: "不通过", State_css: "status-rejected"}
		},
		"Date": func(t time.Time) string {
			return fmt.Sprintf("%d-%d-%d", t.Year(), t.Month(), t.Day())
		},
		"reviewerTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return fmt.Sprintf("%d-%d-%d %d-%d-%d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
		},
	})
	t, err = t.Parse(string(b))
	if err != nil {
		panic(err)
	}
	return t
}()

var t2 = func() *template.Template {
	b, err := os.ReadFile("./afl_html/review.temp")
	if err != nil {
		panic(err)
	}
	t := template.New("")
	t = t.Funcs(template.FuncMap{
		"getReviewResult": func(r *Request) ReviewResult {
			if r.Reviewer == "" {
				return ReviewResult{State: "未审批", State_css: "badge-pending"}
			}
			if r.Approve {
				return ReviewResult{State: "已批准", State_css: "badge-approved"}
			}
			return ReviewResult{State: "已拒绝", State_css: "badge-rejected"}
		},
		"Date": func(t time.Time) string {
			return fmt.Sprintf("%d-%d-%d", t.Year(), t.Month(), t.Day())
		},
		"reviewerTime": func(t time.Time) string {
			return fmt.Sprintf("%d-%d-%d %d-%d-%d", t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
		},
	})
	t, err = t.Parse(string(b))
	if err != nil {
		panic(err)
	}
	return t
}()

var t3 = func() *template.Template {
	b, err := os.ReadFile("./afl_html/result_id.temp")
	if err != nil {
		panic(err)
	}
	t := template.New("")
	t, err = t.Parse(string(b))
	if err != nil {
		panic(err)
	}
	return t
}()

type ReviewResult struct {
	State, State_css string
}

func GetAll(name string) *Review {
	r := Root{Name: name}
	d := Review{Root: r}
	result := db.Find(&d.All)
	if result.Error != nil {
		panic(result.Error)
	}
	d.Count.Total = len(d.All)
	for _, r := range d.All {
		if r.Reviewer == "" {
			d.Count.Pending += 1
			d.Pending = append(d.Pending, r)
		} else if r.Approve {
			d.Count.Approved += 1
		} else {
			d.Count.Refuse += 1
		}
	}
	return &d
}

var c = safesession.NewControl(
	key.Encrypt, key.Decrypt,
	2*365*24*time.Hour,
	http.SameSiteLaxMode,
	func(clientIp string) safesession.IPInfo {
		//TODO:实现这里
		return safesession.IPInfo{}
	},
	safesession.DB{
		Store: func(ID string, CreateTime time.Time) bool {
			//TODO:实现清除过期会话
			//Note:在创建用户或登录时，更新用户信息，已经保存了，这里不需要重复保存。
			return true
		},
		Delete: func(ID string) {
			result := db.Model(&Root{}).Where("session_id = ?", ID).Update("session_id", "")
			if result.Error != gorm.ErrRecordNotFound && result.Error != nil {
				panic(result.Error)
			}
		},
		Exist: func(ID string) bool {
			r := Root{SessionID: ID}
			result := db.Where(&r).Take(&r)
			if result.Error != nil {
				panic(result.Error)
			}
			return r.Name != ""
		},
		Valid: func(UserName, SessionID string) error {
			r := Root{Name: UserName}
			result := db.Where(&r).Take(&r)
			if result.Error != nil {
				panic(result.Error)
			}
			if r.SessionID == SessionID {
				return nil
			}
			return errors.New("未登录或登录已失效")
		},
	},
)
